// Package utils contains various supplementary functions and data structures.
// This file vm.go contains functions related to hypervisors and VMs management functions.
package utils

import (
	"archive/zip"
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
)

// ProgressWrapper type is used to track download progress.
type ProgressWrapper struct {
	io.Reader
	total    int64
	size     int64
	progress float64
	step     float64
}

// Md5Wrapper type is used to calculate file's md5 sum during download.
type Md5Wrapper struct {
	io.Writer
	md5sum hash.Hash
}

func (pw *ProgressWrapper) Read(p []byte) (int, error) {
	n, err := pw.Reader.Read(p)
	if n > 0 {
		pw.total += int64(n)
		progress := float64(pw.total) / float64(pw.size) * float64(100)
		// Show progress for each N%
		if progress-pw.progress > pw.step {
			fmt.Printf("Downloaded %.2f%%\r", progress)
			pw.progress = progress
		} else if pw.total == pw.size {
			fmt.Println("Download finished")
		}
	}
	return n, err
}

func (mw *Md5Wrapper) Write(p []byte) (int, error) {
	n, err := mw.Writer.Write(p)
	mw.md5sum.Write(p)
	return n, err
}

// getOrigMd5 function gets MD5 provided by Microsoft for each VM archive.
func getOrigMd5(vm VMImage) string {
	resp, err := http.Get(vm.Md5URL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	origMd5, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return string(origMd5)
}

func compareMd5(md5str1, md5str2 string) {
	if md5str1 != md5str2 {
		fmt.Println("MD5 sum doesn't match. Aborting.")
		os.Exit(1)
	} else {
		fmt.Println("MD5 sum matches.")
	}
}

// DownloadVM function downloads VM archive defined by a user and returns the path where it was stored.
func DownloadVM(uc UserChoice) string {
	// TODO: during download add .part extension to the downloaded file
	// TODO: add check for .part file for resumable downloads
	// TODO: return error instead of panic()
	vmFile := path.Join(uc.DownloadPath, path.Base(uc.VMImage.FileURL))
	fmt.Printf("Prepare to download %s to %s\n", uc.VMImage.FileURL, vmFile)

	origMd5 := getOrigMd5(uc.VMImage)
	fmt.Printf("Expected MD5 sum %s\n", origMd5)

	if _, err := os.Stat(vmFile); err == nil {
		fmt.Printf("File %s already exists.\nChecking MD5 sum\n", vmFile)
		oldFile, err := os.Open(vmFile)
		if err != nil {
			panic(err)
		}
		defer oldFile.Close()

		oldMd5 := md5.New()
		if _, err := io.Copy(oldMd5, oldFile); err != nil {
			panic(err)
		}

		vmMd5 := fmt.Sprintf("%X", oldMd5.Sum([]byte{}))
		fmt.Printf("Local file MD5 sum %s\n", vmMd5)
		compareMd5(origMd5, vmMd5)
	} else {
		fmt.Println("Start downloading.")

		newFile, err := os.Create(vmFile)
		if err != nil {
			panic(err)
		}
		defer newFile.Close()
		newFileMd5 := &Md5Wrapper{Writer: newFile, md5sum: md5.New()}

		resp, err := http.Get(uc.VMImage.FileURL)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		fmt.Printf("File size %d bytes\n", resp.ContentLength)
		vmSrc := &ProgressWrapper{
			Reader: resp.Body,
			size:   resp.ContentLength,
			// progress download step for 1Mb chunks
			step: float64(1024*1024) / float64(resp.ContentLength) * float64(100),
		}

		if _, err := io.Copy(newFileMd5, vmSrc); err != nil {
			panic(err)
		}

		vmMd5 := fmt.Sprintf("%X", newFileMd5.md5sum.Sum([]byte{}))
		fmt.Printf("Downloaded file MD5 sum %s\n", vmMd5)
		compareMd5(origMd5, vmMd5)
	}
	return vmFile
}

// vmFilePath function finds a specific file path depending on a hypervisor.
// Different hypervisors have different file names for VMs. For example, VirtualBox has .ova extension but VMware needs
// .ovf file etc.
func vmFilePath(hypervisor string, collectedPaths []string) string {
	search := ""
	if hypervisor == "VirtualBox" {
		search = ".ova"
	} else if hypervisor == "VMware" {
		search = ".ovf"
	} else {
		fmt.Printf("Hypervisor %s isn't supported.\n", hypervisor)
		return ""
	}
	for _, vmPath := range collectedPaths {
		if strings.HasSuffix(vmPath, search) {
			return vmPath
		}
	}
	// TODO: return error if found nothing
	return ""
}

// UnzipVM function unpack downloaded VM archive.
func UnzipVM(uc UserChoice) string {
	vmPath := path.Join(uc.DownloadPath, path.Base(uc.VMImage.FileURL))
	zipReader, err := zip.OpenReader(vmPath)
	if err != nil {
		panic(err)
	}
	defer zipReader.Close()

	unzipFolder := path.Join(uc.DownloadPath, path.Base(uc.VMImage.FileURL))
	unzipFolderParts := strings.Split(unzipFolder, ".")
	unzipFolder = strings.Join(unzipFolderParts[:len(unzipFolderParts)-1], ".")
	if _, err := os.Stat(unzipFolder); os.IsNotExist(err) {
		if err := os.Mkdir(unzipFolder, 0755); err != nil {
			panic(err)
		}
	}
	fmt.Printf("Unpack data into '%s'\n", unzipFolder)

	var collectedPaths []string
	for _, file := range zipReader.File {
		fmt.Printf("Unpacking '%s'\n", file.Name)
		filePath := path.Join(unzipFolder, file.Name)
		if _, err := os.Stat(filePath); err == nil {
			collectedPaths = append(collectedPaths, filePath)
			fmt.Printf("File '%s' already exist, skip\n", filePath)
			continue
		}
		if file.FileInfo().IsDir() {
			os.MkdirAll(filePath, file.Mode())
			continue
		}

		// Collected paths are required because each hypervisor has its own entry point file.
		// For example, VirtualBox needs .ova file, VMware needs .ovf file and Hyper-V needs .xml file etc.
		collectedPaths = append(collectedPaths, filePath)

		fileReader, err := file.Open()
		if err != nil {
			panic(err)
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_CREATE, file.Mode())
		if err != nil {
			panic(err)
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			panic(err)
		}
	}
	return vmFilePath(uc.Hypervisor, collectedPaths)
}

func virtualboxCheck() error {
	// TODO: improve VirtualBox installation checks for Windows platforms.
	fmt.Println("Checking VirtualBox installation..")
	cmdName := "vboxmanage"
	cmdArgs := []string{"--version"}
	result, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
	if err != nil {
		fmt.Println(string(result), err)
		return err
	}
	fmt.Println("Detected vboxmanage version", string(result))
	return nil
}

func virtualboxImportVM(vmPath string) error {
	// NOTE: vboxmanage can import the same VM many times
	fmt.Println("Import VM into VirtualBox. Please wait.")
	cmdName := "vboxmanage"
	cmdArgs := []string{"import", vmPath}
	result, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
	if err != nil {
		fmt.Println(string(result), err)
		return err
	}
	fmt.Println(string(result))
	return nil
}

func vmwareCheck() error {
	// NOTE: VMware requires two command line tools to works with VMs.
	fmt.Println("Checking VMware installation..")
	cmdName := "ovftool"
	cmdArgs := []string{"--version"}
	result, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
	if err != nil {
		fmt.Println(string(result), err)
		return err
	}
	fmt.Println("Detected", string(result))

	// NOTE: vmrun doesn't have --help or --version or similar options.
	// Without any parameters it exits with status code 255 and help text.
	cmdName = "vmrun"
	result, err = exec.Command(cmdName).CombinedOutput()
	if fmt.Sprintf("%s", err) != "exit status 255" {
		fmt.Println(string(result), err)
		return err
	}
	fmt.Println("Detected", strings.Split(string(result), "\n")[1])
	return nil
}

// vmwareConvert functions converts provided .ovf file into .vmx file.
func vmwareConvert(ovfPath string) (string, error) {
	// NOTE: ovftool fails if .vmx file exists
	vmxPath := strings.Replace(ovfPath, ".ovf", ".vmx", 1)
	fmt.Printf("Convert %s to %s. Please wait.\n", ovfPath, vmxPath)

	cmdName := "ovftool"
	cmdArgs := []string{ovfPath, vmxPath}
	result, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
	if err != nil {
		fmt.Println(string(result), err)
		return "", err
	}
	fmt.Println(string(result))
	return vmxPath, nil
}

// vmwareFixNetwork functions add missed network configuration into .vmx file.
func vmwareFixNetwork(vmxPath string) {
	if vmxFile, err := os.Stat(vmxPath); err == nil {
		if vmxFile, err := os.OpenFile(vmxPath, os.O_APPEND|os.O_WRONLY, vmxFile.Mode()); err == nil {
			vmxFile.WriteString("ethernet0.present = \"TRUE\"\n")
			vmxFile.WriteString("ethernet0.connectionType = \"nat\"\n")
			vmxFile.WriteString("ethernet0.wakeOnPcktRcv = \"FALSE\"\n")
			vmxFile.WriteString("ethernet0.addressType = \"generated\"\n")
			vmxFile.Close()
		}
	}
}

func vmwareImportVM(vmxPath string) error {
	// NOTE: VMware runvm command doesn't have anything like import, so start and stop sub-commands
	// are used to add a VM into the library.
	fmt.Printf("Starting %s VM\n", vmxPath)

	cmdName := "vmrun"
	cmdArgs := []string{"start", vmxPath}
	if _, err := exec.Command(cmdName, cmdArgs...).Output(); err != nil {
		return err
	}

	fmt.Printf("Stopping %s VM\n", vmxPath)
	cmdArgs[0] = "stop"
	if _, err := exec.Command(cmdName, cmdArgs...).Output(); err != nil {
		return err
	}
	return nil
}

// InstallVM function installs unpacked VM into a selected hypervisor.
func InstallVM(hypervisor string, vmPath string) {
	if hypervisor == "VirtualBox" {
		if err := virtualboxCheck(); err == nil {
			virtualboxImportVM(vmPath)
		}
	} else if hypervisor == "VMware" {
		if err := vmwareCheck(); err != nil {
			os.Exit(1)
		}
		vmxPath, err := vmwareConvert(vmPath)
		if err != nil {
			os.Exit(1)
		}
		vmwareFixNetwork(vmxPath)
		vmwareImportVM(vmxPath)
	} else {
		fmt.Printf("Hypervisor %s isn't supported.\n", hypervisor)
	}
}
