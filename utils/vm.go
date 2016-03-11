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
	"runtime"
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

func pathJoin(path1, path2 string) string {
	if runtime.GOOS == "windows" {
		return strings.Replace(path.Join(path1, path2), "/", "\\", -1)
	}
	return path.Join(path1, path2)
}

// DownloadVM function downloads VM archive defined by a user and returns the path where it was stored.
func DownloadVM(uc UserChoice) string {
	vmFile := pathJoin(uc.DownloadPath, path.Base(uc.VMImage.FileURL))
	fmt.Printf("Download: %s\nTo: %s\n", uc.VMImage.FileURL, vmFile)

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
func vmFilePath(hypervisor string, collectedPaths []string) (string, error) {
	search := ""
	switch hypervisor {
	case "VirtualBox":
		search = ".ova"
	case "VMware":
		search = ".ovf"
	case "HyperV":
		search = ".xml"
	case "Parallels":
		search = ".pvs"
	}
	if search != "" {
		for _, vmPath := range collectedPaths {
			if strings.HasSuffix(vmPath, search) {
				return vmPath, nil
			}
		}
	}
	return "", fmt.Errorf("Din't find VM path for %s\n", hypervisor)
}

// UnzipVM function unpack downloaded VM archive.
func UnzipVM(uc UserChoice) (string, error) {
	vmPath := pathJoin(uc.DownloadPath, path.Base(uc.VMImage.FileURL))
	zipReader, err := zip.OpenReader(vmPath)
	if err != nil {
		return "", err
	}
	defer zipReader.Close()

	unzipFolder := pathJoin(uc.DownloadPath, path.Base(uc.VMImage.FileURL))
	unzipFolderParts := strings.Split(unzipFolder, ".")
	unzipFolder = strings.Join(unzipFolderParts[:len(unzipFolderParts)-1], ".")
	if _, err := os.Stat(unzipFolder); os.IsNotExist(err) {
		if err := os.Mkdir(unzipFolder, 0755); err != nil {
			return "", err
		}
	}
	fmt.Printf("Unpack data into '%s'\n", unzipFolder)

	var collectedPaths []string
	for _, file := range zipReader.File {
		fmt.Printf("Unpacking '%s'\n", file.Name)
		filePath := pathJoin(unzipFolder, file.Name)
		if _, err := os.Stat(filePath); err == nil {
			collectedPaths = append(collectedPaths, filePath)
			fmt.Printf("File '%s' already exist, skip.\n", filePath)
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
			return "", err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_CREATE, file.Mode())
		if err != nil {
			return "", err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return "", err
		}
	}
	return vmFilePath(uc.Hypervisor, collectedPaths)
}

func checkVirtualBox() error {
	// TODO: improve VirtualBox installation checks for Windows platforms.
	fmt.Println("Checking VirtualBox installation.")
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

func importVirtualBoxVM(vmPath string) error {
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

func checkVmware() error {
	// TODO: improve VMware installation checks for Windows platforms.
	// NOTE: VMware requires two command line tools to works with VMs.
	fmt.Println("Checking VMware installation.")
	cmdName := "ovftool"
	cmdArgs := []string{"--version"}
	result, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
	if err != nil {
		fmt.Println(string(result), err)
		return err
	}
	fmt.Println("Detected", string(result))

	// NOTE: vmrun doesn't have --help or --version or similar options.
	// Without any parameters it exits with status code 255 (Linux, Mac)
	// or 4294967295 (Windows) and shows help text. So command execution
	// output is checked to determine if vmrun is present.
	cmdName = "vmrun"
	result, err = exec.Command(cmdName).CombinedOutput()
	if len(result) < 2 {
		fmt.Println(string(result), err)
		return err
	}

	version := strings.Split(string(result), "\n")[1]
	if !strings.Contains(version, "vmrun version") {
		fmt.Println(string(result), err)
		return err
	}
	fmt.Println("Detected", version)
	return nil
}

// convertVmware function converts provided .ovf file into .vmx file.
func convertVmware(ovfPath string) (string, error) {
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

// fixVmwareNetwork function adds missed network configuration into .vmx file.
func fixVmwareNetwork(vmxPath string) {
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

func importVmwareVM(vmxPath string) error {
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

func checkHyperv() error {
	// Powershell is required for Hyper-V.
	fmt.Println("Checking Hyper-V installation.")
	cmdName := "powershell"
	cmdArgs1 := []string{"-Command", "Get-Host"}
	if result, err := exec.Command(cmdName, cmdArgs1...).CombinedOutput(); err != nil {
		fmt.Println(string(result))
		return err
	}
	fmt.Println("Powershell is present.")

	// Check if Hyper-V Cmdlets are available.
	cmdArgs2 := []string{"-Command", "Get-Command", "-Module", "Hyper-V"}
	if result, err := exec.Command(cmdName, cmdArgs2...).CombinedOutput(); err != nil {
		fmt.Println(string(result))
		return err
	}
	fmt.Println("Hyper-V Cmdlets are present.")
	return nil
}

func importHypervVM(vmPath string) error {
	fmt.Printf("Import '%s'. Please wait.\n", vmPath)
	cmdName := "powershell"
	cmdArgs1 := []string{"-Command", "Import-VM", "-Path", fmt.Sprintf("'%s'", vmPath)}
	if result, err := exec.Command(cmdName, cmdArgs1...).CombinedOutput(); err != nil {
		fmt.Println(string(result))
		return err
	}
	// NOTE: Hyper-V uses virtual network switches for VMs. After installation it doesn't have any network switches
	// set. Also it could have several virtual network switches. So the imported VM is left as-is and a user should
	// configure networking manually.
	fmt.Println("WARNING: Please check Network adapter settings. By default it isn't connected.")
	return nil
}

func checkParallels() error {
	// NOTE: Parallels has two command line tools prlsrvctl and prlctl.
	// Parallels version could be checked with prlsrvctl but VM management is done with prlctl.
	fmt.Println("Checking Parallels installation.")
	cmdName := "prlsrvctl"
	cmdArgs := []string{"info"}
	result, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
	if err != nil {
		fmt.Println(string(result), err)
		return err
	}
	fmt.Println(string(result))
	return nil
}

func importParallelsVM(vmPath string) error {
	fmt.Println("Import VM into Parallels. Please wait.")
	cmdName := "prlctl"
	cmdArgs := []string{"register", vmPath}
	result, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
	if err != nil {
		fmt.Println(string(result), err)
		return err
	}
	fmt.Println(string(result))
	return nil
}

// InstallVM function installs unpacked VM into a selected hypervisor.
func InstallVM(hypervisor string, vmPath string) {
	switch hypervisor {
	case "VirtualBox":
		if err := checkVirtualBox(); err == nil {
			importVirtualBoxVM(vmPath)
		}
	case "VMware":
		if err := checkVmware(); err == nil {
			if vmxPath, err := convertVmware(vmPath); err == nil {
				fixVmwareNetwork(vmxPath)
				importVmwareVM(vmxPath)
			}
		}
	case "HyperV":
		if err := checkHyperv(); err == nil {
			importHypervVM(vmPath)
		}
	case "Parallels":
		fmt.Println(vmPath)
		if err := checkParallels(); err == nil {
			importParallelsVM(vmPath)
		}
	default:
		fmt.Printf("Hypervisor %s isn't supported.\n", hypervisor)
	}
}
