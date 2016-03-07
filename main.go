package main

import (
	"./utils"
	"os"
)

var BuildRev string

const VMS_URL = "https://dev.windows.com/en-us/microsoft-edge/tools/vms/windows/"

func main() {
	utils.ShowBanner(BuildRev)
	args := os.Args[1:]
	if len(args) == 0 {
		raw_data := utils.DownloadJson(VMS_URL)
		platforms, hypervisors, browsers, available_vms := utils.ParseJson(&raw_data)

		user_choice := utils.UserChoice{}
		user_choice.Spec.Platform = utils.SelectOption(
			platforms, "Select platform", "All", utils.GetDefaultPlatform)
		user_choice.Spec.Hypervisor = utils.SelectOption(
			hypervisors, "Select hypervisor", user_choice.Spec.Platform, utils.GetDefaultHypervisor)
		user_choice.Spec.BrowserOs = utils.SelectOption(
			browsers, "Select browser and OS", user_choice.Spec.Hypervisor, utils.GetDefaultBrowser)
		user_choice.VmImage = available_vms[user_choice.Spec]
		user_choice.DownloadPath = utils.SelectOption(
			utils.GetDownloadPaths(), "Select download path", "All", utils.GetDefaultDownloadPath)
		utils.ConfirmUsersChoice(user_choice)

		vm_archive := utils.DownloadVm(user_choice)
		vm_path := utils.UnzipVm(vm_archive)
		utils.InstallVm(user_choice.Spec.Hypervisor, vm_path)
	} else {
		// TODO: process command-line args
	}
}
