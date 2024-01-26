// nomad-onload
// Copyright (c) 2024 Neomantra BV

package onload_device

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/structs"
)

// Each fingerprint response consists of either an error or a list of device groups.
// A "device group" is a list of detected devices that are identical for the purpose of scheduling;
// that is, they will have identical attributes.
//   - https://developer.hashicorp.com/nomad/docs/concepts/plugins/devices
//
// In our model, we make each "GroupNameKey" of "<vendor>/<device_type>/<model>" its own "device group"

// Task config parameter device.name :
//   <device_type>
//   <vendor>/<device_type>
//   <vendor>/<device_type>/<model>

// FingerprintDeviceData is a device record from fingerprinting
type FingerprintDeviceData struct {
	Interface  string // also its Name
	DeviceType string
	Vendor     string
	Model      string
	PCIBusID   string
}

func (d *FingerprintDeviceData) GroupNameKey() string {
	return fmt.Sprintf("%s/%s/%s", d.Vendor, d.DeviceType, d.Model)
}

// FingerprintData represets attributes of driver/devices
type FingerprintData struct {
	Devices   []*FingerprintDeviceData
	OOVersion string // OpenOnload (OO) version
	ZFVersion string // TCPDirect (ZF) version
}

func (d *OnloadDevicePlugin) getFingerprintData() (*FingerprintData, error) {
	// "discover" Onload and any NICs
	// This may change dynamically, if Onload is installed while the Nomad agent is running
	ooVersion, err := ProbeOnloadVersion(d.config.HostOnloadBinPath)
	if err != nil {
		d.logger.Info("Onload not found", "err", err.Error())
	}

	zfVersion, err := ProbeZFVersion(d.config.HostZfBinPath)
	if err != nil {
		d.logger.Info("TCPDirect not found", "err", err.Error())
	}

	var deviceInfos []DeviceInfo
	if d.config.ProbeSFC {
		devs, err := ProbeOnloadSFCNics()
		if err != nil {
			d.logger.Info("Issue probing SFC NICs", "err", err.Error())
		}
		deviceInfos = append(deviceInfos, devs...)
	}
	if d.config.ProbeXDP {
		devs, err := ProbeOnloadXDPNics()
		if err != nil {
			d.logger.Info("Issue probing XDP NICs", "err", err.Error())
		}
		deviceInfos = append(deviceInfos, devs...)
	}
	if len(deviceInfos) == 0 {
		// if we did not discover any SFC or XDP NIC,s that's OK.
		// Onload can be used without it, so we publish
		// a fake device called "none" to still allow Onload enablement
		// via the "<device_type>" name configuration
		deviceInfos = append(deviceInfos, DeviceInfo{
			Interface: deviceName_None,
			Vendor:    vendor_None,
			PCIBusID:  "",
		})
	}

	// list of eligble Onload/ZF device types
	var deviceTypes []string
	if ooVersion != "" {
		deviceTypes = append(deviceTypes, deviceType_Onload)
		if zfVersion != "" {
			deviceTypes = append(deviceTypes, deviceType_ZF, deviceType_OnloadZF)
		}
	}

	// create the fingerprint device list
	devices := make([]*FingerprintDeviceData, 0, len(deviceTypes)*len(deviceInfos))
	for _, dev := range deviceInfos {
		for _, deviceType := range deviceTypes {
			// create pseudo-devices for non-exclusive access
			d.logger.Info("Fingerprinted NIC device", "deviceType", deviceType, "iface", dev.Interface)
			devices = append(devices, makePsuedoDeviceFingerprints(d.config.NumPsuedoNIC, deviceType, dev)...)
		}
	}

	// Now lets handle Timekeeping
	if d.config.ProbePPS {
		if devs, err := ProbePPS(); err == nil {
			for _, dev := range devs {
				d.logger.Info("Fingerprinted PPS device", "deviceType", deviceType_PPS, "iface", dev.Interface)
				devices = append(devices, makePsuedoDeviceFingerprints(d.config.NumPsuedoPPS, deviceType_PPS, dev)...)
			}
		} else {
			d.logger.Info("Issue probing PPS devices", "err", err.Error())
		}
	}
	if d.config.ProbePTP {
		if devs, err := ProbePTP(); err == nil {
			for _, dev := range devs {
				d.logger.Info("Fingerprinted PTP device", "deviceType", deviceType_PTP, "iface", dev.Interface)
				devices = append(devices, makePsuedoDeviceFingerprints(d.config.NumPsuedoPTP, deviceType_PTP, dev)...)
			}
		} else {
			d.logger.Info("Issue probing PTP devices", "err", err.Error())
		}
	}

	// Return the Fingerprint data
	return &FingerprintData{
		OOVersion: ooVersion,
		ZFVersion: zfVersion,
		Devices:   devices,
	}, nil
}

// Creates pseudo-device fingerprints for non-exclusive access to a device. DeviceID = "<interface>-<pdev-num>", like "eth0-0"
func makePsuedoDeviceFingerprints(numPsuedoDevices int, deviceType string, devInfo DeviceInfo) []*FingerprintDeviceData {
	var fingprintDevices []*FingerprintDeviceData
	for i := 0; i < numPsuedoDevices; i++ {
		deviceID := fmt.Sprintf("%s-%d", devInfo.Interface, i)
		fingprintDevices = append(fingprintDevices, &FingerprintDeviceData{
			Interface:  deviceID,
			Model:      devInfo.Interface, // hard to know actual Model, so use Interface as specifier
			DeviceType: deviceType,
			Vendor:     devInfo.Vendor,
			PCIBusID:   devInfo.PCIBusID,
		})
	}
	return fingprintDevices
}

///////////////////////////////////////////////////////////////////////////////

// doFingerprint is the long-running goroutine that detects device changes
func (d *OnloadDevicePlugin) doFingerprint(ctx context.Context, devices chan *device.FingerprintResponse) {
	defer close(devices)

	// Create a timer that will fire immediately for the first detection
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(d.fingerprintPeriod)
		}

		d.writeFingerprintToChannel(devices)
	}
}

// writeFingerprintToChannel collects fingerprint info, partitions network devices into
// "device groups" (by Interface name), and sends the data over the provided channel.
func (d *OnloadDevicePlugin) writeFingerprintToChannel(devices chan<- *device.FingerprintResponse) {
	fingerprintData, err := d.getFingerprintData()
	if err != nil {
		d.logger.Error("failed to fingerprint onload devices", "error", err)
		devices <- device.NewFingerprintError(err)
		return
	}
	d.logger.Debug("fingerprint results", "len_devices", len(fingerprintData.Devices), "oo", fingerprintData.OOVersion, "zf", fingerprintData.ZFVersion)

	// exclude ignored interfaces
	fingerprintDevices := ignoreFingerprintedDevices(fingerprintData.Devices, d.ignoredInterfaces)

	// check if any device health was updated or any device was added to host
	if !d.fingerprintChanged(fingerprintDevices) {
		return
	}

	// Build common attributes
	commonAttributes := map[string]*structs.Attribute{
		attr_OnloadVersion: {
			String: pointer.Of(fingerprintData.OOVersion),
		},
		attr_ZFVersion: {
			String: pointer.Of(fingerprintData.ZFVersion),
		},
	}

	// Group all FingerprintDevices by Interface attribute
	deviceListByGroupNameKey := make(map[string][]*FingerprintDeviceData)
	for _, device := range fingerprintData.Devices {
		key := device.GroupNameKey()
		if key == "" {
			key = groupName_NotAvailable
		}
		deviceListByGroupNameKey[key] = append(deviceListByGroupNameKey[key], device)
	}

	// Build Fingerprint response with computed groups and send it over the channel
	deviceGroups := make([]*device.DeviceGroup, 0, len(deviceListByGroupNameKey))
	for groupName, devices := range deviceListByGroupNameKey {
		deviceGroups = append(deviceGroups, d.deviceGroupFromFingerprintData(groupName, devices, commonAttributes))
	}
	devices <- device.NewFingerprint(deviceGroups...)
}

// ignoreFingerprintedDevices excludes ignored devices from fingerprint output
func ignoreFingerprintedDevices(deviceData []*FingerprintDeviceData, ignoredInterfaces map[string]string) []*FingerprintDeviceData {
	var result []*FingerprintDeviceData
	for _, fingerprintDevice := range deviceData {
		if _, ignored := ignoredInterfaces[fingerprintDevice.Interface]; !ignored {
			result = append(result, fingerprintDevice)
		}
	}
	return result
}

// fingerprintChanged checks if there are any previously unseen Onload devices located
// or any of fingerprinted Onload devices disappeared since the last fingerprint run.
// Also, this func updates the device map on OnloadDevicePlugin with the latest data
func (d *OnloadDevicePlugin) fingerprintChanged(allDevices []*FingerprintDeviceData) bool {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	changeDetected := false
	// check if every device in allDevices is in d.devices
	for _, device := range allDevices {
		if _, ok := d.devices[device.Interface]; !ok {
			changeDetected = true
		}
	}

	// check if every device in d.devices is in allDevices
	fingerprintDeviceMap := make(map[string]*FingerprintDeviceData)
	for _, device := range allDevices {
		fingerprintDeviceMap[device.Interface] = device
	}
	for id := range d.devices {
		if _, ok := fingerprintDeviceMap[id]; !ok {
			changeDetected = true
		}
	}

	d.devices = fingerprintDeviceMap
	return changeDetected
}

// deviceGroupFromFingerprintData composes deviceGroup from FingerprintDeviceData slice
func (d *OnloadDevicePlugin) deviceGroupFromFingerprintData(groupName string, deviceList []*FingerprintDeviceData, commonAttributes map[string]*structs.Attribute) *device.DeviceGroup {
	// deviceGroup without devices makes no sense -> return nil when no devices are provided
	if len(deviceList) == 0 {
		return nil
	}

	devices := make([]*device.Device, 0, len(deviceList))
	for _, dev := range deviceList {
		devices = append(devices, &device.Device{
			ID: dev.Interface,
			// all fingerprinted devices are "healthy" for now
			Healthy:    true,
			HealthDesc: "",
			HwLocality: &device.DeviceLocality{
				PciBusID: dev.PCIBusID, // This helps the NUMA-aware scheduler =)
			},
		})
	}

	dev := deviceList[0] // safe, we checked above
	deviceGroup := &device.DeviceGroup{
		Vendor:     dev.Vendor,
		Type:       dev.DeviceType,
		Name:       dev.Model,
		Devices:    devices,
		Attributes: map[string]*structs.Attribute{},
	}

	// Extend attribute map with common attributes
	for attributeKey, attributeValue := range commonAttributes {
		deviceGroup.Attributes[attributeKey] = attributeValue
	}

	return deviceGroup
}
