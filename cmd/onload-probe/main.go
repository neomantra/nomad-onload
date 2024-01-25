// nomad-onload
// Copyright (c) 2024 Neomantra BV

package main

import (
	"fmt"
	"os"

	device "github.com/neomantra/nomad-device-onload/internal/onload_device"
	"github.com/spf13/pflag"
)

func main() {

	var showHelp bool
	var onloadDir string

	pflag.StringVarP(&onloadDir, "dir", "d", "/usr/bin", "Directory holding the onload executable")
	pflag.BoolVar(&showHelp, "help", false, "Show help")
	pflag.Parse()

	if showHelp {
		fmt.Fprintf(os.Stdout, "usage: %s\n", os.Args[0])
		pflag.PrintDefaults()
		os.Exit(0)
	}

	ooVersion, err := device.ProbeOnloadVersion(onloadDir)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Onload version: not found (err: %s)\n", err.Error())
	} else {
		fmt.Fprintf(os.Stdout, "Onload version: %s\n", ooVersion)
	}

	zfVersion, err := device.ProbeZFVersion(onloadDir)
	if err != nil {
		fmt.Fprintf(os.Stdout, "TCPDirect version: not found (err: %s)\n", err.Error())
	} else {
		fmt.Fprintf(os.Stdout, "TCPDirect version: %s\n", zfVersion)
	}

	fmt.Fprintf(os.Stdout, "Onload hardware-accelerated interfaces:\n")
	sfcNics, err := device.ProbeOnloadSFCNics()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query SFC interfaces: %s\n", err.Error())
	} else {
		for _, nic := range sfcNics {
			fmt.Fprintf(os.Stdout, "  %-8s %s\n", nic.Interface, nic.PCIBusID)
		}
	}

	fmt.Fprintf(os.Stdout, "XDP hardware-accelerated interfaces: (FAKE, ROADMAP)\n")
	if xdpNics, err := device.ProbeOnloadXDPNics(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query XDP interfaces: %s\n", err.Error())
	} else {
		for _, nic := range xdpNics {
			fmt.Fprintf(os.Stdout, "  %-8s %s\n", nic.Interface, nic.PCIBusID)
		}
	}

	fmt.Fprintf(os.Stdout, "PPS devices:\n")
	if ppsDevs, err := device.ProbePPS(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query PPS devices: %s\n", err.Error())
	} else {
		for _, nic := range ppsDevs {
			fmt.Fprintf(os.Stdout, "  %-8s %s\n", nic.Interface, nic.PCIBusID)
		}
	}

	fmt.Fprintf(os.Stdout, "PTP devices:\n")
	if ppsDevs, err := device.ProbePTP(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query PTP devices: %s\n", err.Error())
	} else {
		for _, nic := range ppsDevs {
			fmt.Fprintf(os.Stdout, "  %-8s %s\n", nic.Interface, nic.PCIBusID)
		}
	}
}
