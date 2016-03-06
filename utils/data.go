// Data related functions
package utils

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"regexp"
	"encoding/json"
"strings"
	"runtime"
	"path"
	"os"
)

// JsonData represents data obtained by DownloadJson function
// some fields, like _ts, _etag, __colId etc are omitted
type JsonData struct {
	Active       bool   `json:"active"`
	ID           string `json:"id"`
	ReleaseNotes string `json:"releaseNotes"`
	SoftwareList []struct {
		OsList       []string `json:"osList"`
		SoftwareName string   `json:"softwareName"`
		Vms          []struct {
			BrowserName string `json:"browserName"`
			Build       string `json:"build"`
			Files       []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
				Md5  string `json:"md5,omitempty"`
			} `json:"files"`
			OsVersion string `json:"osVersion"`
			Version   string `json:"version"`
		} `json:"vms"`
	} `json:"softwareList"`
	Version string `json:"version"`
}

// Available choices
type Choice map[int]string

// Choices grouped by other choices
type ChoiceGroups map[string]Choice

// Spec defines possible configuration
type Spec struct {
	Platform   string
	Hypervisor string
	// IE version and Windows version available as a single option
	BrowserOs  string
}

// VmImage defines from where download virtual machine image and its md5 sum
type VmImage struct {
	FileURL string
	// instead of actual md5 sum value an URL to a file which contains md5 value is provided
	Md5URL     string
}

// Available VMs for given Specs
type AvailableVms map[Spec]VmImage

// Parameters selected by a user
type UserChoice struct {
	Spec
	VmImage
	DownloadPath string
}

// Default option selector function
type DefaultChoice func(choices Choice) int

func DownloadJson(page_url string) []byte {
	// In truth this page is regular HTML page but it has JSON data which is
	// used to build IE version selection menus, so the whole page is downloaded
	// and parsed by regexp to extract the actual JSON.
	fmt.Printf("Download JSON data from %s\n\n", page_url)
	resp, err := http.Get(page_url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile("vms = (.*?);")
	return re.FindSubmatch(body)[1]
}

// Parse extracted JSON into more convenient data structures
func ParseJson(raw_data *[]byte) (
	platforms, hypervisors, browsers ChoiceGroups, available_vms AvailableVms) {
	var data JsonData
	if err := json.Unmarshal(*raw_data, &data); err != nil {
		panic(err)
	}

	seen_platforms := make(map[string]bool)
	platforms = make(ChoiceGroups)
	hypervisors = make(ChoiceGroups)
	browsers = make(ChoiceGroups)
	available_vms = make(AvailableVms)

	for h_idx, software := range data.SoftwareList {
		hypervisor := software.SoftwareName
		if hypervisor == "Vagrant" {
			// skip Vagrant because it isn't a hypervisor
			continue
		}

		for os_idx, platform := range software.OsList {
			_, ok1 := platforms["All"]
			if !ok1 {
				platforms["All"] = make(Choice)
			}
			_, ok2 := seen_platforms[platform]
			if !ok2 {
				seen_platforms[platform] = true
				platforms["All"][os_idx] = platform
			}
			_, ok3 := hypervisors[platform]
			if !ok3 {
				hypervisors[platform] = make(Choice)
			}
			hypervisors[platform][h_idx] = hypervisor
		}

		for b_idx, browser := range software.Vms {
			browser_os := strings.Join([]string{browser.BrowserName, browser.OsVersion}, " ")
			_, ok := browsers[hypervisor]
			if !ok {
				browsers[hypervisor] = make(Choice)
			}
			browsers[hypervisor][b_idx] = browser_os
			for _, file := range browser.Files {
				if file.Md5 != "" {
					vm := VmImage{FileURL: file.URL, Md5URL: file.Md5}
					for _, p := range software.OsList {
						spec := Spec{Platform: p, Hypervisor: hypervisor, BrowserOs: browser_os}
						available_vms[spec] = vm
					}
				}
			}
		}
	}

	return platforms, hypervisors, browsers, available_vms
}

// Get default download path based on OS
func get_download_path() string {
	switch runtime.GOOS {
	case "linux":
		return path.Join(os.Getenv("HOME"), "Downloads")
	case "darwin":
		return path.Join(os.Getenv("HOME"), "Downloads")
	case "windows":
		return path.Join(os.Getenv("USERPROFILE"), "Downloads")
	default:
		return ""
	}
}

func get_working_path() string {
	working_path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return working_path
}


func GetDownloadPaths() ChoiceGroups {
	choices := make(ChoiceGroups)
	choices["All"] = make(Choice)
	for path_idx, download_path := range []string{get_working_path(), get_download_path()} {
		if download_path != "" {
			choices["All"][path_idx+1] = download_path
		}
	}
	return choices
}

func GetDefaultPlatform(choices Choice) int {
	for idx, platform := range choices {
		switch {
		case platform == "Linux" && runtime.GOOS == "linux":
			return idx
		case platform == "Windows" && runtime.GOOS == "windows":
			return idx
		case platform == "Mac" && runtime.GOOS == "darwin":
			return idx
		}
	}
	return 1
}

func GetDefaultHypervisor(choices Choice) int {
	for idx, hypervisor := range choices {
		if hypervisor == "VirtualBox" {
			return idx
		}
	}
	return 1
}

func GetDefaultBrowser(choices Choice) int {
	// Consider the latest browser is the newest
	return len(choices)
}

func GetDefaultDownloadPath(choices Choice) int {
	for idx, download_path := range choices {
		if strings.Contains(download_path, "Downloads") {
			return idx
		}
	}
	return len(choices)
}
