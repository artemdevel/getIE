// VM related functions
package utils

import (
	"io"
	"hash"
	"fmt"
	"net/http"
	"io/ioutil"
	"os"
	"path"
	"crypto/md5"
	"archive/zip"
	"os/exec"
	"syscall"
)

const DOWNLOAD_STEP = 0.5

// Wrapper to track download progress
type ProgressWrapper struct {
	io.Reader
	total    int64
	size     int64
	progress float64
}

// Wrapper to calculate md5 sum of the downloaded
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
		if progress- pw.progress > DOWNLOAD_STEP {
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

// Get MD5 provided by Microsoft for each VM archive
func get_orig_md5(vm VmImage) string {
	resp, err := http.Get(vm.Md5URL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	orig_md5, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return string(orig_md5)
}

func compare_md5(md5str1, md5str2 string) {
	if md5str1 != md5str2 {
		fmt.Println("MD5 sum doesn't match. Exiting.")
		os.Exit(1)
	} else {
		fmt.Println("MD5 sum matches.")
	}
}

// Download VM archive and return path where it was stored
func DownloadVm(uc UserChoice) string {
	// TODO: during download add .part extension to the downloaded file
	// TODO: add check for .part file for resumable downloads
	vm_file := path.Join(uc.DownloadPath, path.Base(uc.VmImage.FileURL))
	fmt.Printf("Prepare to download %s to %s\n", uc.VmImage.FileURL, vm_file)

	orig_md5 := get_orig_md5(uc.VmImage)
	fmt.Printf("Expected MD5 sum %s\n", orig_md5)

	if _, err := os.Stat(vm_file); err == nil {
		fmt.Printf("File %s already exists.\nChecking MD5 sum\n", vm_file)
		old_file, err := os.Open(vm_file)
		if err != nil {
			panic(err)
		}
		defer old_file.Close()

		old_md5 := md5.New()
		if _, err := io.Copy(old_md5, old_file); err != nil {
			panic(err)
		}

		vm_md5 := fmt.Sprintf("%X", old_md5.Sum([]byte{}))
		fmt.Printf("Local file MD5 sum %s\n", vm_md5)
		compare_md5(orig_md5, vm_md5)
	} else {
		fmt.Println("Start downloading.")

		new_file, err := os.Create(vm_file)
		if err != nil {
			panic(err)
		}
		defer new_file.Close()
		new_file_md5 := &Md5Wrapper{Writer: new_file, md5sum: md5.New()}

		vm_resp, err := http.Get(uc.VmImage.FileURL)
		if err != nil {
			panic(err)
		}
		defer vm_resp.Body.Close()
		fmt.Printf("File size %d bytes\n", vm_resp.ContentLength)
		vm_src := &ProgressWrapper{Reader: vm_resp.Body, size: vm_resp.ContentLength}

		if _, err := io.Copy(new_file_md5, vm_src); err != nil {
			panic(err)
		}

		vm_md5 := fmt.Sprintf("%X", new_file_md5.md5sum.Sum([]byte{}))
		fmt.Printf("Downloaded file MD5 sum %s\n", vm_md5)
		compare_md5(orig_md5, vm_md5)
	}
	EnterToContinue()
	return vm_file
}

// Unzip downloaded VM
func UnzipVm(vm_path string) string {
	// TODO: check if destination folder exists
	// TODO: support folders and multiple files because VirtualBox has a single file
	// TODO: the code must be rewritten to unpack all files into some folder and this folder must be returned
	// TODO: each hypervisor requires its own specific files as a start point so just a folder path isn't enough
	zip_file, err := zip.OpenReader(vm_path)
	if err != nil {
		panic(err)
	}
	defer zip_file.Close()

	dst_path := path.Dir(vm_path)
	for _, file := range zip_file.File {
		file_reader, err := file.Open()
		if err != nil {
			panic(err)
		}
		defer file_reader.Close()

		fmt.Printf("Unpacking '%s' to %s\n", file.Name, dst_path)
		dst_file := path.Join(dst_path, file.Name)
		vm_file, err := os.Create(dst_file)
		if err != nil {
			panic(err)
		}
		defer vm_file.Close()


		if _, err := io.Copy(vm_file, file_reader); err != nil {
			panic(err)
		}
		fmt.Println("Unpacking finished")
		EnterToContinue()
		return dst_file
	}
	return ""
}

func check_virtualbox() {
	cmd_name := "vboxmanage"
	cmd_args := []string{"--version"}
	if version, err := exec.Command(cmd_name, cmd_args...).Output(); err != nil {
		panic(err)
	} else {
		fmt.Println("Detected VirtualBox version", string(version))
	}
}

func virtualbox_list_vms() {
	cmd_name := "vboxmanage"
	cmd_args := []string{"list", "vms"}
	if cmd_output, err := exec.Command(cmd_name, cmd_args...).Output(); err != nil {
		panic(err)
	} else {
		fmt.Println("Existed VirtualBox VMs")
		fmt.Println(string(cmd_output))
	}
}

func virtualbox_import_vm(vm_path string) {
	binary, err := exec.LookPath("vboxmanage")
	if err != nil {
		panic(err)
	}

	cmd_args := []string{"vboxmanage", "import", vm_path}
	if err := syscall.Exec(binary, cmd_args, os.Environ()); err != nil {
		panic(err)
	}
}

// Install unpacked VM into selected hypervisor
func InstallVm(vm_hypervisor string, vm_path string) {
	switch vm_hypervisor {
	case "VirtualBox":
		check_virtualbox()
		virtualbox_list_vms()
		EnterToContinue()
		virtualbox_import_vm(vm_path)
	case "HyperV":
		fmt.Println("HyperV hypervisor isn't supported yet. Exiting..")
	case "VPC":
		fmt.Println("VPC hypervisor isn't supported yet. Exiting..")
	case "VMware":
		fmt.Println("VMware hypervisor isn't supported yet. Exiting..")
	case "Parallels":
		fmt.Println("Parallels hypervisor isn't supported yet. Exiting..")
	default:
		fmt.Printf("Unknown hypervisor %s. Exiting..\n", vm_hypervisor)
	}
}
