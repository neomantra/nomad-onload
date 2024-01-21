// nomad-onload
// Copyright (c) 2024 Neomantra BV

package main

import (
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins"

	device "github.com/neomantra/nomad-device-onload/internal/onload_device"
)

func main() {
	// Serve the plugin
	plugins.Serve(factory)
}

// factory returns a new instance of our example device plugin
func factory(log log.Logger) interface{} {
	return device.NewOnloadDevicePlugin(log)
}
