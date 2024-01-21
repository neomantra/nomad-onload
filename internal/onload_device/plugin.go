// nomad-onload
// Copyright (c) 2024 Neomantra BV

package onload_device

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/kr/pretty"
)

const (
	// pluginName is the deviceName of the plugin
	// this is used for logging and (along with the version) for uniquely identifying
	// plugin binaries fingerprinted by the client
	pluginName = "onload"

	// plugin version allows the client to identify and use newer versions of
	// an installed plugin
	pluginVersion = "v0.1.0"

	// vendor is the label for the vendor providing the devices.
	// along with "type" and "model", this can be used when requesting devices:
	//   https://www.nomadproject.io/docs/job-specification/device.html#name
	//
	// SFC devices were created by Solaflare, which was acquired by Xilinx,
	// which was acquired by AMD.  We will assign "amd" to "sfc" devices.
	vendor_SFC = "amd"
	// XDP driver discovery?
	vendor_XDP = "xdp"

	// deviceType is the "type" of device being returned
	deviceType_Onload   = "onload"
	deviceType_ZF       = "zf"
	deviceType_OnloadZF = "onloadzf"

	groupName_notAvailable = "NA"
	deviceName_none        = "none"

	// attribute names
	attr_OnloadVersion = "onload_version"
	attr_ZFVersion     = "zf_version"
)

///////////////////////////////////////////////////////////////////////////////

type configDesc struct {
	// name, type, required, default, docs
	Name     string
	Type     string
	Required bool
	Default  string
	Docs     string
}

// Config contains configuration information for the plugin.
type OnloadDevicePluginConfig struct {
	NeedNic            bool     `codec:"need_nic"`
	SetPreload         bool     `codec:"set_preload"`
	MountOnload        bool     `codec:"mount_onload"`
	IgnoredInterfaces  []string `codec:"ignored_interfaces"`
	TaskDevicePath     string   `codec:"task_device_path"`
	HostDevicePath     string   `codec:"host_device_path"`
	TaskOnloadBinPath  string   `codec:"task_onload_bin_path"`
	HostOnloadBinPath  string   `codec:"host_onload_bin_path"`
	TaskOnloadLibPath  string   `codec:"task_onload_lib_path"`
	HostOnloadLibPath  string   `codec:"host_onload_lib_path"`
	TaskProfileDirPath string   `codec:"task_profile_dir_path"`
	HostProfileDirPath string   `codec:"host_profile_dir_path"`
	TaskZfBinPath      string   `codec:"task_zf_bin_path"`
	HostZfBinPath      string   `codec:"host_zf_bin_path"`
	TaskZfLibPath      string   `codec:"task_zf_lib_path"`
	HostZfLibPath      string   `codec:"host_zf_lib_path"`
	FingerprintPeriod  string   `codec:"fingerprint_period"`
}

var (
	// pluginInfo provides information used by Nomad to identify the plugin
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDevice,
		PluginApiVersions: []string{device.ApiVersion010},
		PluginVersion:     pluginVersion,
		Name:              pluginName,
	}

	// configDescriptions is converted into configSpec, the specification of the schema for this plugin's config.
	configDescriptions = []configDesc{
		{"need_nic", "bool", false, `true`, "Should the Device Plugin fail if no compatible nics are found"},
		{"set_preload", "bool", false, `true`, "Should the device plugin set the LD_PRELOAD environment variable in the Task"},
		{"mount_onload", "bool", false, `false`, "Should the device plugin mount Onload files into the Nomad Task"},
		{"ignored_interfaces", "list(string)", false, `[]`, "List of interfaces to ignore.  Include `none` to prevent that pseudo-devices creation"},
		{"task_device_path", "string", false, `"/dev"`, "Path to place device files in the Nomad Task"},
		{"host_device_path", "string", false, `"/dev"`, "Path to find device files on the Host"},
		{"task_onload_lib_path", "string", false, `"/usr/lib/x86_64-linux-gnu"`, "Path to place Onload libraries in the Nomad Task"},
		{"host_onload_lib_path", "string", false, `"/usr/lib/x86_64-linux-gnu"`, "Path to find Onload libraries on the Host"},
		{"task_onload_bin_path", "string", false, `"/usr/bin"`, "Path to place Onload binaries in the Nomad Task"},
		{"host_onload_bin_path", "string", false, `"/usr/bin"`, "Path to find Onload binaries on the Host"},
		{"task_profile_dir_path", "string", false, `"/usr/libexec/onload/profiles"`, "Path to place Onload profiles directory in the Nomad Task"},
		{"host_profile_dir_path", "string", false, `"/usr/libexec/onload/profiles"`, "Path to find Onload profiles directory on the Host"},
		{"task_zf_bin_path", "string", false, `"/usr/bin"`, "Path to place TCPDirect/ZF binaries in the Nomad Task"},
		{"host_zf_bin_path", "string", false, `"/usr/bin"`, "Path to find TCPDirect/ZF binaries on the Host"},
		{"task_zf_lib_path", "string", false, `"/usr/lib/x86_64-linux-gnu"`, "Path to place TCPDirect/ZF libraries in the Nomad Task"},
		{"host_zf_lib_path", "string", false, `"/usr/lib/x86_64-linux-gnu"`, "Path to find TCPDirect/ZF libraries on the Host"},
		{"fingerprint_period", "string", false, `"1m"`, "Period of time between attemps to fingerpint devices"},
	}
)

///////////////////////////////////////////////////////////////////////////////

// OnloadDevicePlugin is a Nomad Device Plugin exposing Onload devices.
type OnloadDevicePlugin struct {
	logger log.Logger

	// these are local copies of the config values that we need for operation
	config OnloadDevicePluginConfig

	// fingerprintPeriod the period for the fingerprinting loop
	// most plugins that fingerprint in a polling loop will have this
	fingerprintPeriod time.Duration

	// ignoredInterfaces is a set of Interfaces that would not be exposed to Nomad
	ignoredInterfaces map[string]string

	// devices is a list of fingerprinted devices
	devices    map[string]*FingerprintDeviceData
	deviceLock sync.RWMutex
}

// NewPlugin returns a device plugin, used primarily by the main wrapper
//
// Plugin configuration isn't available yet, so there will typically be
// a limit to the initialization that can be performed at this point.
func NewOnloadDevicePlugin(log log.Logger) *OnloadDevicePlugin {
	return &OnloadDevicePlugin{
		logger:            log.Named(pluginName),
		ignoredInterfaces: make(map[string]string),
		devices:           make(map[string]*FingerprintDeviceData),
	}
}

// PluginInfo returns information describing the plugin.
//
// This is called during Nomad client startup, while discovering and loading
// plugins.
func (d *OnloadDevicePlugin) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the configuration schema for the plugin.
//
// This is called during Nomad client startup, immediately before parsing
// plugin config and calling SetConfig
func (d *OnloadDevicePlugin) ConfigSchema() (*hclspec.Spec, error) {
	// this is used to validate the HCL for the plugin provided
	// as part of the client config:
	//   https://www.nomadproject.io/docs/configuration/plugin.html
	// options are here:
	//   https://github.com/hashicorp/nomad/blob/v0.10.0/plugins/shared/hclspec/hcl_spec.proto
	//
	// These configs are adapted from here:
	//   https://github.com/Xilinx-CNS/kubernetes-onload/blob/master/cmd/deviceplugin/main.go
	configSpec := map[string]*hclspec.Spec{}
	for _, desc := range configDescriptions {
		var spec *hclspec.Spec
		if desc.Default != "" {
			spec = hclspec.NewDefault(
				hclspec.NewAttr(desc.Name, desc.Type, desc.Required),
				hclspec.NewLiteral(desc.Default))
		} else {
			spec = hclspec.NewAttr(desc.Name, desc.Type, desc.Required)
		}
		configSpec[desc.Name] = spec
	}
	return hclspec.NewObject(configSpec), nil
}

// SetConfig is called by the client to pass the configuration for the plugin.
func (d *OnloadDevicePlugin) SetConfig(c *base.Config) error {
	// decode the plugin config
	var config OnloadDevicePluginConfig
	if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
		return err
	}

	// save the configuration to the plugin
	d.config = config

	// convert the fingerprint poll period from an HCL string into a time.Duration
	period, err := time.ParseDuration(config.FingerprintPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse doFingerprint period %q: %v", config.FingerprintPeriod, err)
	}
	d.fingerprintPeriod = period

	// convert d.config.ignoredInterfaces array to d.ignoredInterfaces map
	for _, ignoredInterface := range config.IgnoredInterfaces {
		d.ignoredInterfaces[ignoredInterface] = ignoredInterface
	}

	d.logger.Info("config set", "config", log.Fmt("% #v", pretty.Formatter(config)))
	return nil
}

// Fingerprint streams detected devices.
// Messages should be emitted to the returned channel when there are changes
// to the devices or their health.
func (d *OnloadDevicePlugin) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	// Fingerprint returns a channel. The recommended way of organizing a plugin
	// is to pass that into a long-running goroutine and return the channel immediately.
	outCh := make(chan *device.FingerprintResponse)
	go d.doFingerprint(ctx, outCh)
	return outCh, nil
}

// Stats streams statistics for the detected devices.
// Messages should be emitted to the returned channel on the specified interval.
func (d *OnloadDevicePlugin) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	// Similar to Fingerprint, Stats returns a channel. The recommended way of
	// organizing a plugin is to pass that into a long-running goroutine and
	// return the channel immediately.
	outCh := make(chan *device.StatsResponse)
	go d.doStats(ctx, outCh, interval)
	return outCh, nil
}
