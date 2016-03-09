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
type Choice []string

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


func (ch Choice) Len() int { return len(ch) }
func (ch Choice) Less(i, j int) bool { return ch[i] < ch[j] }
func (ch Choice) Swap(i, j int) { ch[i], ch[j] = ch[j], ch[i] }

func DownloadJson(page_url string) []byte {
	// page_url contains JSON which is used to build IE version selection menus,
	// so the whole page is downloaded and parsed by regexp to extract the actual JSON.
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

	for _, software := range data.SoftwareList {
		hypervisor := software.SoftwareName
		if hypervisor == "Vagrant" {
			// skip Vagrant because it isn't a hypervisor
			continue
		}

		for _, platform := range software.OsList {
			if !seen_platforms[platform] {
				seen_platforms[platform] = true
				platforms["All"] = append(platforms["All"], platform)
			}
			hypervisors[platform] = append(hypervisors[platform], hypervisor)
		}

		for _, browser := range software.Vms {
			browser_os := strings.Join([]string{browser.BrowserName, browser.OsVersion}, " ")
			browsers[hypervisor] = append(browsers[hypervisor], browser_os)
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
	for _, download_path := range []string{get_working_path(), get_download_path()} {
		if download_path != "" {
			choices["All"] = append(choices["All"], download_path)
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
	return len(choices) - 1
}

func GetDefaultDownloadPath(choices Choice) int {
	for idx, download_path := range choices {
		if strings.Contains(download_path, "Downloads") {
			return idx
		}
	}
	return len(choices)
}
