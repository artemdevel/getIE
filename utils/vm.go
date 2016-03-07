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
	"strings"
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
		fmt.Println("MD5 sum doesn't match. Aborting.")
		os.Exit(1)
	} else {
		fmt.Println("MD5 sum matches.")
	}
}

// Download VM archive and return path where it was stored
func DownloadVm(uc UserChoice) string {
	// TODO: during download add .part extension to the downloaded file
	// TODO: add check for .part file for resumable downloads
	// TODO: return error instead of panic()
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
	return vm_file
}

// Get exact file path depending on a hypervisor
func vm_file_path(hypervisor string, collected_paths []string) string {
	search := ""
	if hypervisor == "VirtualBox" {
		search = ".ova"
	} else {
		fmt.Printf("Hypervisor %s isn't supported.\n", hypervisor)
		return ""
	}
	for _, vm_path := range collected_paths {
		if strings.HasSuffix(vm_path, search) {
			return vm_path
		}
	}
	// TODO: return error if found nothing
	return ""
}

// Unzip downloaded VM
func UnzipVm(uc UserChoice) string {
	vm_path := path.Join(uc.DownloadPath, path.Base(uc.VmImage.FileURL))
	zip_reader, err := zip.OpenReader(vm_path)
	if err != nil {
		panic(err)
	}
	defer zip_reader.Close()

	unzip_folder := path.Join(uc.DownloadPath, path.Base(uc.VmImage.FileURL))
	unzip_folder_parts := strings.Split(unzip_folder, ".")
	unzip_folder = strings.Join(unzip_folder_parts[:len(unzip_folder_parts)-1], ".")
	if _, err := os.Stat(unzip_folder); os.IsNotExist(err) {
		if err := os.Mkdir(unzip_folder, 0755); err != nil {
			panic(err)
		}
	}
	fmt.Printf("Unpack data into %s\n", unzip_folder)

	var collected_paths []string
	for _, file := range zip_reader.File {
		fmt.Printf("Unpacking '%s'\n", file.Name)
		file_path := path.Join(unzip_folder, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(file_path, file.Mode())
			continue
		}

		// Collected paths are required because each hypervisor has its own entry point file.
		// For example, VirtualBox needs .ova file, VMware needs .vmdk file and Hyper-V needs .xml file etc
		collected_paths = append(collected_paths, file_path)

		file_reader, err := file.Open()
		if err != nil {
			panic(err)
		}
		defer file_reader.Close()

		target_file, err := os.OpenFile(file_path, os.O_WRONLY|os.O_CREATE|os.O_CREATE, file.Mode())
		if err != nil {
			panic(err)
		}
		defer target_file.Close()


		if _, err := io.Copy(target_file, file_reader); err != nil {
			panic(err)
		}
	}
	return vm_file_path(uc.Hypervisor, collected_paths)
}

func check_virtualbox() error {
	cmd_name := "vboxmanage"
	cmd_args := []string{"--version"}
	if version, err := exec.Command(cmd_name, cmd_args...).Output(); err != nil {
		fmt.Println("VirtualBox not found. Aborting.")
		return err
	} else {
		fmt.Println("Detected VirtualBox version", string(version))
	}
	return nil
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
func InstallVm(hypervisor string, vm_path string) {
	if hypervisor == "VirtualBox" {
		if err := check_virtualbox(); err == nil {
			virtualbox_import_vm(vm_path)
		}
	} else {
		fmt.Printf("Hypervisor %s isn't supported.\n", hypervisor)
	}
}
