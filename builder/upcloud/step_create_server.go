package upcloud

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/packer-plugin-upcloud/internal/driver"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/packerbuilderdata"
)

// StepCreateServer represents the step that creates a server
type StepCreateServer struct {
	Config        *Config
	GeneratedData *packerbuilderdata.GeneratedData
}

// Run runs the actual step
func (s *StepCreateServer) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	drv := state.Get("driver").(driver.Driver)

	rawSshKeyPublic, ok := state.GetOk("ssh_key_public")
	if !ok {
		return stepHaltWithError(state, fmt.Errorf("SSH public key is missing"))
	}
	sshKeyPublic := rawSshKeyPublic.(string)

	ui.Say("Getting storage...")

	storage, err := drv.GetStorage(s.Config.StorageUUID, s.Config.StorageName)
	if err != nil {
		return stepHaltWithError(state, err)
	}

	ui.Say(fmt.Sprintf("Creating server based on storage %q...", storage.Title))

	networking := DefaultNetworking
	if len(s.Config.NetworkInterfaces) > 0 {
		networking = convertNetworkTypes(s.Config.NetworkInterfaces)

	}
	response, err := drv.CreateServer(&driver.ServerOpts{
		StorageUuid:  storage.UUID,
		StorageSize:  s.Config.StorageSize,
		Zone:         s.Config.Zone,
		SshPublicKey: sshKeyPublic,
		Networking:   networking,
	})
	if err != nil {
		return stepHaltWithError(state, err)
	}

	serverUuid := response.UUID
	serverTitle := response.Title
	serverIp, err := getServerIp(response)
	if err != nil {
		return stepHaltWithError(state, err)
	}

	ui.Say(fmt.Sprintf("Server %q created and in 'started' state", serverTitle))

	state.Put("server_uuid", serverUuid)
	state.Put("server_title", serverTitle)
	state.Put("server_ip", serverIp)

	s.GeneratedData.Put("ServerUUID", serverUuid)
	s.GeneratedData.Put("ServerTitle", serverTitle)
	s.GeneratedData.Put("ServerSize", serverIp)

	return multistep.ActionContinue
}

// Cleanup stops and destroys the server if server details are found in the state
func (s *StepCreateServer) Cleanup(state multistep.StateBag) {
	// Extract server uuid, return if no uuid has been stored
	rawServerUuid, ok := state.GetOk("server_uuid")

	if !ok {
		return
	}

	serverUuid := rawServerUuid.(string)
	serverTitle := state.Get("server_title").(string)

	ui := state.Get("ui").(packer.Ui)
	driver := state.Get("driver").(driver.Driver)

	// stop server
	ui.Say(fmt.Sprintf("Stopping server %q...", serverTitle))

	err := driver.StopServer(serverUuid)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	// delete server
	ui.Say(fmt.Sprintf("Deleting server %q...", serverTitle))

	err = driver.DeleteServer(serverUuid)
	if err != nil {
		ui.Error(err.Error())
		return
	}
}
