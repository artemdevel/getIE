package main

import (
	"./utils"
	"os"
)

// BuildRev var is set from the command line and used in ShowBanner function.
var BuildRev string

const vmsURL = "https://dev.windows.com/en-us/microsoft-edge/tools/vms/windows/"

func main() {
	utils.ShowBanner(BuildRev)
	args := os.Args[1:]
	if len(args) == 0 {
		rawData := utils.DownloadJSON(vmsURL)
		platforms, hypervisors, browsers, availableVms := utils.ParseJSON(&rawData)

		userChoice := utils.UserChoice{}
		userChoice.Platform = utils.SelectOption(
			platforms, "Select platform", "All", utils.GetDefaultPlatform)
		userChoice.Hypervisor = utils.SelectOption(
			hypervisors, "Select hypervisor", userChoice.Platform, utils.GetDefaultHypervisor)
		userChoice.BrowserOs = utils.SelectOption(
			browsers, "Select browser and OS", userChoice.Hypervisor, utils.GetDefaultBrowser)
		userChoice.VMImage = availableVms[userChoice.Spec]
		userChoice.DownloadPath = utils.SelectOption(
			utils.GetDownloadPaths(), "Select download path", "All", utils.GetDefaultDownloadPath)
		utils.ConfirmUsersChoice(userChoice)

		utils.DownloadVM(userChoice)
		utils.EnterToContinue("Download finished")

		vmPath := utils.UnzipVM(userChoice)
		utils.EnterToContinue("Unzip finished")

		utils.InstallVM(userChoice.Hypervisor, vmPath)
	} else {
		// TODO: process command-line args
	}
}
