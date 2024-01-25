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

// onloadDependFiles are the dependencies of the onload command, copied directly from host to target
var onloadDependFiles = []string{
	"/sbin/lsmod",
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

	// Initialize the response
	resp := &device.ContainerReservation{
		Envs:    map[string]string{},
		Mounts:  []*device.Mount{},
		Devices: []*device.DeviceSpec{},
	}
	// Add devices
	for _, deviceID := range deviceIDs {
		device, ok := d.devices[deviceID]
		if !ok {
			d.logger.Warn("Reserving a device not known", "deviceID", deviceID)
			continue
		}

		switch device.DeviceType {
		case deviceType_Onload, deviceType_ZF, deviceType_OnloadZF:
			// updates resp
			d.logger.Info("Reserving onload device", "deviceID", deviceID, "deviceType", device.DeviceType)
			d.reserveOnloadDevice(resp, device.DeviceType, deviceID)
		case deviceType_PTP, deviceType_PPS:
			d.logger.Info("Reserving timekeeping device", "deviceID", deviceID, "deviceType", device.DeviceType)
			d.reserveTimekeepingDevice(resp, device.DeviceType, device.Model)
		default:
			d.logger.Warn("Reserving a DeviceType not known", "deviceType", device.DeviceType, "deviceID", deviceID)
			continue
		}

	}
	return resp, nil
}

///////////////////////////////////////////////////////////////////////////////

func (d *OnloadDevicePlugin) reserveOnloadDevice(resp *device.ContainerReservation, deviceType string, deviceID string) {
	// Always mount the Devices
	if d.config.TaskDevicePath != "" && d.config.HostDevicePath != "" {
		deviceFiles := onloadDeviceFiles
		if deviceType == deviceType_ZF {
			deviceFiles = zfDeviceFiles
		}
		for _, deviceFile := range deviceFiles {
			resp.Devices = append(resp.Devices, &device.DeviceSpec{
				TaskPath:    path.Join(d.config.TaskDevicePath, deviceFile),
				HostPath:    path.Join(d.config.HostDevicePath, deviceFile),
				CgroupPerms: "mrw",
			})
		}
	}

	// Always mount the Libraries
	if d.config.TaskOnloadLibPath != "" && d.config.HostOnloadLibPath != "" {
		for _, libName := range onloadLibraryFiles {
			resp.Mounts = append(resp.Mounts, &device.Mount{
				TaskPath: path.Join(d.config.TaskOnloadLibPath, libName),
				HostPath: path.Join(d.config.HostOnloadLibPath, libName),
				ReadOnly: true,
			})
		}
	}
	if (deviceType == deviceType_ZF) || (deviceType == deviceType_OnloadZF) {
		for _, libName := range zfLibraryFiles {
			resp.Mounts = append(resp.Mounts, &device.Mount{
				TaskPath: path.Join(d.config.TaskOnloadLibPath, libName),
				HostPath: path.Join(d.config.HostOnloadLibPath, libName),
				ReadOnly: true,
			})
		}
	}

	// Copy the Userspace executables and profiles into the container?
	if d.config.MountOnload {
		// Onload executables and profiles
		if d.config.TaskOnloadBinPath != "" && d.config.HostOnloadBinPath != "" {
			for _, binName := range onloadBinaryFiles {
				resp.Mounts = append(resp.Mounts, &device.Mount{
					TaskPath: path.Join(d.config.TaskOnloadBinPath, binName),
					HostPath: path.Join(d.config.HostOnloadBinPath, binName),
					ReadOnly: true,
				})
			}
			for _, depName := range onloadDependFiles {
				resp.Mounts = append(resp.Mounts, &device.Mount{
					TaskPath: depName,
					HostPath: depName,
					ReadOnly: true,
				})
			}
		}
		if d.config.TaskProfileDirPath != "" && d.config.HostProfileDirPath != "" {
			resp.Mounts = append(resp.Mounts, &device.Mount{
				TaskPath: d.config.TaskProfileDirPath,
				HostPath: d.config.HostProfileDirPath,
				ReadOnly: true,
			})
		}

		// ZF / TCPDirect executables
		if (deviceType == deviceType_ZF) || (deviceType == deviceType_OnloadZF) {
			for _, binName := range zfBinaryFiles {
				resp.Mounts = append(resp.Mounts, &device.Mount{
					TaskPath: path.Join(d.config.TaskOnloadBinPath, binName),
					HostPath: path.Join(d.config.HostOnloadBinPath, binName),
					ReadOnly: true,
				})
			}
		}
	}

	// Setup LD_PRELOAD if desired, but not if we are a "zf" deviceType
	if d.config.SetPreload && deviceType != deviceType_ZF && d.config.TaskOnloadLibPath != "" {
		resp.Envs["LD_PRELOAD"] = path.Join(d.config.TaskOnloadLibPath, onloadPreloadFile)
	}
}

///////////////////////////////////////////////////////////////////////////////

func (d *OnloadDevicePlugin) reserveTimekeepingDevice(resp *device.ContainerReservation, deviceType string, deviceInterface string) {
	resp.Devices = append(resp.Devices, &device.DeviceSpec{
		TaskPath:    path.Join(d.config.TaskDevicePath, deviceInterface),
		HostPath:    path.Join(d.config.HostDevicePath, deviceInterface),
		CgroupPerms: "mrw",
	})
}
