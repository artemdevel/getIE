package main

import (
	"./utils"
	"fmt"
)

// BuildRev var is set from the command line and used in ShowBanner function to indicate build revision.
var BuildRev string

const vmsURL = "https://dev.windows.com/en-us/microsoft-edge/tools/vms/windows/"

func main() {
	utils.ShowBanner(BuildRev)

	rawData := utils.DownloadJSON(vmsURL)
	platforms, hypervisors, browsers, availableVms := utils.ParseJSON(&rawData)

	userChoice := utils.UserChoice{}
	userChoice.Platform = utils.SelectOption(platforms, "Select platform", "All", utils.GetDefaultPlatform)
	userChoice.Hypervisor = utils.SelectOption(hypervisors, "Select hypervisor", userChoice.Platform, utils.GetDefaultHypervisor)
	utils.ShowHypervisorWarning(userChoice.Hypervisor)
	userChoice.BrowserOs = utils.SelectOption(browsers, "Select browser and OS", userChoice.Hypervisor, utils.GetDefaultBrowser)
	userChoice.VMImage = availableVms[userChoice.Spec]
	userChoice.DownloadPath = utils.SelectOption(utils.GetDownloadPaths(), "Select download path", "All", utils.GetDefaultDownloadPath)
	utils.ConfirmUsersChoice(userChoice)

	utils.DownloadVM(userChoice)
	utils.EnterToContinue("Download finished.")
	if vmPath, err := utils.UnzipVM(userChoice); err == nil {
		utils.EnterToContinue("Unzip finished.")
		utils.InstallVM(userChoice.Hypervisor, vmPath)
	} else {
		fmt.Println(err)
	}
}
