// Package utils contains various supplementary functions and data structures.
// This file data.go contains functions related to data processing and defines all data structures.
package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
)

// JSONData represents data obtained by DownloadJson function.
// Some fields, like _ts, _etag, __colId etc are omitted.
type JSONData struct {
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

// Choice type represents list of available choices.
type Choice []string

// ChoiceGroups type represents list of choices grouped by some string key (other choices).
type ChoiceGroups map[string]Choice

// Spec type defines possible configuration available for a user to choose from.
type Spec struct {
	Platform   string
	Hypervisor string
	// IE version and Windows version available as a single option.
	BrowserOs string
}

// VMImage type defines VM archive file metadata.
type VMImage struct {
	FileURL string
	// Instead of actual md5 sum value Microsoft provides an URL to a file which contains md5 value.
	Md5URL string
}

// AvailableVM type represents VMs available for a given Spec.
type AvailableVM map[Spec]VMImage

// UserChoice type defines options selected by a user.
type UserChoice struct {
	Spec
	VMImage
	DownloadPath string
}

// DefaultChoice type defines a function type which is used to calculate default option index.
type DefaultChoice func(choices Choice) int

// Implement Sort interface to make list of choices sortable.
func (ch Choice) Len() int           { return len(ch) }
func (ch Choice) Less(i, j int) bool { return ch[i] < ch[j] }
func (ch Choice) Swap(i, j int)      { ch[i], ch[j] = ch[j], ch[i] }

// DownloadJSON function downloads given page and extract JSON structure from it.
func DownloadJSON(pageURL string) []byte {
	fmt.Printf("Download JSON data from %s\n\n", pageURL)
	resp, err := http.Get(pageURL)
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

// ParseJSON function parses extracted JSON into more convenient data structures.
func ParseJSON(rawData *[]byte) (
	platforms, hypervisors, browsers ChoiceGroups, availableVms AvailableVM) {
	var data JSONData
	if err := json.Unmarshal(*rawData, &data); err != nil {
		panic(err)
	}

	seenPlatforms := make(map[string]bool)
	platforms = make(ChoiceGroups)
	hypervisors = make(ChoiceGroups)
	browsers = make(ChoiceGroups)
	availableVms = make(AvailableVM)

	for _, software := range data.SoftwareList {
		hypervisor := software.SoftwareName
		if hypervisor == "Vagrant" {
			// skip Vagrant because it isn't a hypervisor
			continue
		}

		for _, platform := range software.OsList {
			if !seenPlatforms[platform] {
				seenPlatforms[platform] = true
				platforms["All"] = append(platforms["All"], platform)
			}
			hypervisors[platform] = append(hypervisors[platform], hypervisor)
		}

		for _, browser := range software.Vms {
			browserOs := strings.Join([]string{browser.BrowserName, browser.OsVersion}, " ")
			browsers[hypervisor] = append(browsers[hypervisor], browserOs)
			for _, file := range browser.Files {
				if file.Md5 != "" {
					vm := VMImage{FileURL: file.URL, Md5URL: file.Md5}
					for _, p := range software.OsList {
						spec := Spec{Platform: p, Hypervisor: hypervisor, BrowserOs: browserOs}
						availableVms[spec] = vm
					}
				}
			}
		}
	}

	return platforms, hypervisors, browsers, availableVms
}

// getDownloadPath function constructs default download path based on OS.
func getDownloadPath() string {
	switch runtime.GOOS {
	case "linux":
		return pathJoin(os.Getenv("HOME"), "Downloads")
	case "darwin":
		return pathJoin(os.Getenv("HOME"), "Downloads")
	case "windows":
		return pathJoin(os.Getenv("USERPROFILE"), "Downloads")
	default:
		return ""
	}
}

// getWorkingPath function returns current working path.
func getWorkingPath() string {
	workingPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return workingPath
}

// GetDownloadPaths function builds a list of choices for available download paths.
func GetDownloadPaths() ChoiceGroups {
	choices := make(ChoiceGroups)
	for _, downloadPath := range []string{getWorkingPath(), getDownloadPath()} {
		if downloadPath != "" {
			choices["All"] = append(choices["All"], downloadPath)
		}
	}
	return choices
}

// GetDefaultPlatform function return an index for current platform from the platforms choices list.
// If no platform detected the first choice is returned (choice indexes are zero-based).
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
	return 0
}

// GetDefaultHypervisor function returns an index for default hypervisor from the hypervisors choices list.
// VirtualBox is now default selection for all platforms but it could be platform specific in the future.
func GetDefaultHypervisor(choices Choice) int {
	for idx, hypervisor := range choices {
		if hypervisor == "VirtualBox" {
			return idx
		}
	}
	return 1
}

// GetDefaultBrowser function returns an index for default browser.
// The latest browser from the list is considered default for now.
func GetDefaultBrowser(choices Choice) int {
	return len(choices) - 1
}

// GetDefaultDownloadPath function returns an index for default download folder.
// User's specific download folder is considered default for now.
func GetDefaultDownloadPath(choices Choice) int {
	for idx, downloadPath := range choices {
		if strings.Contains(downloadPath, "Downloads") {
			return idx
		}
	}
	return len(choices)
}
