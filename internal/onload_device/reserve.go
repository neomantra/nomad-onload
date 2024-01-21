// nomad-onload
// Copyright (c) 2024 Neomantra BV

package onload_device

import (
	"fmt"
	"path"
	"strings"

	"github.com/hashicorp/nomad/plugins/device"
)

// Adapted from:
//  https://github.com/Xilinx-CNS/kubernetes-onload/blob/master/pkg/deviceplugin/mounts.go
//  https://github.com/neomantra/docker-onload

// onloadDeviceFiles are the device files required to run Onload in a container
var onloadDeviceFiles = []string{
	"onload",
	"onload_epoll",
	"sfc_char",
}

// onloadLibraryFiles are the shared libraries required to run Onload in a container
var onloadLibraryFiles = []string{
	"libonload.so",
	"libonload_ext.so",
	// for onload_stackdump:
	// TODO: separate config
	"libpcap.so.0.8",
	"libdbus-1.so.3",
}

// onloadBinaryFiles are the files to be mounted if the user wants to use onload as a script (not using LD_PRELOAD)
var onloadBinaryFiles = []string{
	"onload",
	"onload_stackdump",
}

// onloadPreloadFile is the library file used by LD_PRELOAD
const onloadPreloadFile = "libonload.so"

// zfDeviceFiles are the device files required to run EF/ZF in a container
var zfDeviceFiles = []string{
	"sfc_char",
}

// zfLibraryFiles are the shared libraries required to run ZF in a container
var zfLibraryFiles = []string{
	"libonload_zf.so",
}

// zfBinaryFiles are the files to be mounted if the user wants to use onload as a
// script (not using LD_PRELOAD)
var zfBinaryFiles = []string{
	"zf_stackdump",
	"zf_debug",
}

///////////////////////////////////////////////////////////////////////////////

type reservationError struct {
	notExistingIDs []string
}

func (e *reservationError) Error() string {
	return fmt.Sprintf("unknown device IDs: %s", strings.Join(e.notExistingIDs, ","))
}

// Reserve returns information to the task driver on how to mount the given devices.
// It may also perform any device-specific orchestration necessary to prepare the device
// for use. This is called in a pre-start hook on the client, before starting the workload.
func (d *OnloadDevicePlugin) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	// No Devices? No Reservations... RIP AB
	if len(deviceIDs) == 0 {
		d.logger.Info("No onload devices")
		return &device.ContainerReservation{}, nil
	}

	// This pattern can be useful for some drivers to avoid a race condition where a device disappears
	// after being scheduled by the server but before the server gets an update on the fingerprint
	// channel that the device is no longer available.
	d.deviceLock.RLock()
	var notExistingIDs []string
	for _, id := range deviceIDs {
		if _, deviceIDExists := d.devices[id]; !deviceIDExists {
			notExistingIDs = append(notExistingIDs, id)
		}
	}
	d.deviceLock.RUnlock()
	if len(notExistingIDs) != 0 {
		return nil, &reservationError{notExistingIDs}
	}

	// Figure out if we do ZF
	deviceType := d.devices[deviceIDs[0]].DeviceType

	// Initialize the response
	resp := &device.ContainerReservation{
		Envs:    map[string]string{},
		Mounts:  []*device.Mount{},
		Devices: []*device.DeviceSpec{},
	}

	// Mount the Onload Devices
	for _, deviceFile := range onloadDeviceFiles {
		resp.Devices = append(resp.Devices, &device.DeviceSpec{
			TaskPath:    path.Join(d.config.TaskDevicePath, deviceFile),
			HostPath:    path.Join(d.config.HostDevicePath, deviceFile),
			CgroupPerms: "mrw",
		})
	}

	// Copy the Userspace executables and libraries into the container
	d.logger.Info("Reserve", "mount_onload", d.config.MountOnload)
	if d.config.MountOnload {
		// Onload
		for _, libName := range onloadLibraryFiles {
			resp.Mounts = append(resp.Mounts, &device.Mount{
				TaskPath: path.Join(d.config.TaskOnloadLibPath, libName),
				HostPath: path.Join(d.config.HostOnloadLibPath, libName),
				ReadOnly: true,
			})
		}
		for _, binName := range onloadBinaryFiles {
			resp.Mounts = append(resp.Mounts, &device.Mount{
				TaskPath: path.Join(d.config.TaskOnloadBinPath, binName),
				HostPath: path.Join(d.config.HostOnloadBinPath, binName),
				ReadOnly: true,
			})
		}
		// ZF / TCPDirect
		if (deviceType == deviceType_ZF) || (deviceType == deviceType_OnloadZF) {
			for _, libName := range zfLibraryFiles {
				resp.Mounts = append(resp.Mounts, &device.Mount{
					TaskPath: path.Join(d.config.TaskOnloadLibPath, libName),
					HostPath: path.Join(d.config.HostOnloadLibPath, libName),
					ReadOnly: true,
				})
			}
			for _, binName := range zfBinaryFiles {
				resp.Mounts = append(resp.Mounts, &device.Mount{
					TaskPath: path.Join(d.config.TaskOnloadBinPath, binName),
					HostPath: path.Join(d.config.HostOnloadBinPath, binName),
					ReadOnly: true,
				})
			}

		}
	}

	// TODO: Onload Profiles...

	// Setup LD_PRELOAD if desired, but not if we are a "zf" deviceType
	if d.config.SetPreload && deviceType != deviceType_ZF {
		resp.Envs["LD_PRELOAD"] = path.Join(d.config.TaskOnloadLibPath, onloadPreloadFile)
	}

	for _, deviceID := range deviceIDs {
		d.logger.Info("Reserving onload device", "deviceID", deviceID)
	}
	return resp, nil
}
