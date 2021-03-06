// Package utils contains various supplementary functions and data structures.
// This file cli.go contains functions for very simple console interface.
package utils

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// ShowBanner function shows application's greeting banner.
func ShowBanner(rev string) {
	fmt.Printf("Get IE tool. Build rev %s.\n", rev)
}

// YesNoConfirmation function shows Yes/No choice. N is default choice for now.
func YesNoConfirmation(msg string) {
	reader := bufio.NewReader(os.Stdin)
	defer fmt.Println()
	fmt.Printf("%s [y/N]: ", msg)
	text, _ := reader.ReadString('\n')

	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), "y") {
		fmt.Println("Confirmed. Continue operations")
	} else {
		fmt.Println("Cancelled. Exiting..")
		os.Exit(1)
	}
}

// EnterToContinue function shows press ENTER confirmation for a give message.
func EnterToContinue(msg string) {
	reader := bufio.NewReader(os.Stdin)
	if runtime.GOOS == "darwin" {
		fmt.Printf("%s\nPress ENTER to continue CMD-C to abort.\n", msg)
	} else {
		fmt.Printf("%s\nPress ENTER to continue CTRL-C to abort.\n", msg)
	}
	reader.ReadString('\n')
}

// SelectOption function shows simple selection 'menu'.
func SelectOption(choices ChoiceGroups, groupMsg, groupName string, defaultChoiceFunc DefaultChoice) string {
	reader := bufio.NewReader(os.Stdin)
	defer fmt.Println()

	sortedChoices := choices[groupName]
	sort.Sort(sortedChoices)
	defaultChoice := defaultChoiceFunc(sortedChoices)
	for choice, option := range sortedChoices {
		fmt.Println(choice, option)
	}
	for {
		fmt.Printf("%s [%d]: ", groupMsg, defaultChoice)
		text, _ := reader.ReadString('\n')
		if strings.TrimSpace(text) == "" {
			return sortedChoices[defaultChoice]
		}
		selected, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			continue
		}
		if selected < 0 || selected > len(sortedChoices) {
			continue
		}
		return sortedChoices[selected]
	}
}

// ConfirmUsersChoice shows options selected by a user.
func ConfirmUsersChoice(userChoice UserChoice) {
	fmt.Println("Platform:", userChoice.Spec.Platform)
	fmt.Println("Hypervisor:", userChoice.Spec.Hypervisor)
	fmt.Println("Browser and OS:", userChoice.Spec.BrowserOs)
	fmt.Println("Download path:", userChoice.DownloadPath)
	YesNoConfirmation("Confirm your selection")
}

// ShowHypervisorWarning function shows hypervisor specific warnings if any.
func ShowHypervisorWarning(hypervisor string) {
	switch hypervisor {
	case "HyperV":
		EnterToContinue("WARNING: For HyperV you must run this tool as Administrator.")
	case "VMware":
		if runtime.GOOS == "darwin" {
			EnterToContinue("WARNING: At least VMware Fusion must be installed to run this tool correctly.")
		} else {
			EnterToContinue("WARNING: At least VMware Workstation must be installed to run this tool correctly.")
			EnterToContinue("WARNING: VMware hypervisor isn't compatible with Hyper-V hypervisor.")
		}
	case "Parallels":
			EnterToContinue("WARNING: Parallels Desktop for Mac Pro or Business Edition must be installed to run this tool correctly.")
	case "VirtualBox":
		if runtime.GOOS == "windows" {
			EnterToContinue("WARNING: VirtualBox could fail to run selected VM if Hyper-V is also installed.")
		}
	case "VPC":
		EnterToContinue("WARNING: VPC (Virtual-PC) is obsolete.")
	}
}
