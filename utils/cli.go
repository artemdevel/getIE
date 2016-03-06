// Very simple console interface
package utils

import (
	"fmt"
	"os"
	"bufio"
	"strings"
	"runtime"
	"strconv"
)

// Show application's greeting banner
func ShowBanner(rev string) {
	fmt.Printf("Get IE tool. Build rev %s.\n", rev)
}

// Yes/No choice
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

// Press ENTER to continue
func EnterToContinue() {
	reader := bufio.NewReader(os.Stdin)
	if runtime.GOOS == "darwin" {
		fmt.Println("Press ENTER to continue CMD-C to abort")
	} else {
		fmt.Println("Press ENTER to continue CTRL-C to abort")
	}
	reader.ReadString('\n')
}

// Select an option from 'menu'
func SelectOption(choices ChoiceGroups, group_msg, group_name string, default_choice_func DefaultChoice) string {
	reader := bufio.NewReader(os.Stdin)
	defer fmt.Println()
	// normalize choices
	norm_choices := make(map[int]string)
	idx := 1
	for _, option := range choices[group_name] {
		norm_choices[idx] = option
		idx++
	}
	default_choice := default_choice_func(norm_choices)
	for choice, option := range norm_choices {
		fmt.Println(choice, option)
	}
	for {
		fmt.Printf("%s [%d]: ", group_msg, default_choice)
		text, _ := reader.ReadString('\n')
		if text == "\n" {
			return norm_choices[default_choice]
		}
		selected, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			continue
		}
		_, ok := norm_choices[selected]
		if !ok {
			continue
		}
		return norm_choices[selected]
	}
}

func ConfirmUsersChoice(user_choice UserChoice) {
	fmt.Println("Platform:", user_choice.Spec.Platform)
	fmt.Println("Hypervisor:", user_choice.Spec.Hypervisor)
	fmt.Println("Browser and OS:", user_choice.Spec.BrowserOs)
	fmt.Println("Download path:", user_choice.DownloadPath)
	YesNoConfirmation("Confirm your selection")
}