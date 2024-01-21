// nomad-onload
// Copyright (c) 2024 Neomantra BV

package onload_device

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ProbeOnloadVersion probes the system using `onload --version`.
// Returns the version string, or an empty string and error.
// `binPath` is the path to the directory with `onload`
func ProbeOnloadVersion(binPath string) (string, error) {
	// Verify that the onload binary exists
	onloadBinPath := filepath.Join(binPath, "onload")
	if _, err := os.Stat(onloadBinPath); err != nil {
		return "", fmt.Errorf("onload executable not found at '%s'", onloadBinPath)
	}

	// Fetch its version info
	versionBytes, err := exec.Command(onloadBinPath, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("'onload --version' failed %w", err)
	}

	// Extract version number
	r := regexp.MustCompile("^[Oo]nload ([0-9.]*)")
	m := r.FindSubmatch(versionBytes)
	if len(m) != 2 {
		return "", fmt.Errorf("onload output malformed %w", err)
	}
	return string(m[1]), nil
}

// ProbeZFVersion probes the system using `zf_stackdump version`.
// Returns the version string, or an empty string and error.
// `binPath` is the path to the directory with `zf_stackdump`
func ProbeZFVersion(binPath string) (string, error) {
	// Verify that the onload binary exists
	zfBinPath := filepath.Join(binPath, "zf_stackdump")
	if _, err := os.Stat(zfBinPath); err != nil {
		return "", fmt.Errorf("zf_stackdump executable not found at '%s'", zfBinPath)
	}

	// Fetch its version info
	versionBytes, err := exec.Command(zfBinPath, "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("'zf_stackdump version' failed %w", err)
	}

	// Extract version number
	r := regexp.MustCompile("^TCPDirect Library version: ([0-9.]*)")
	m := r.FindSubmatch(versionBytes)
	if len(m) != 2 {
		return "", fmt.Errorf("zf_stackdump output malformed %w", err)
	}
	return string(m[1]), nil
}

///////////////////////////////////////////////////////////////////////////////

// Original from Kubernetes Onload:
//   https://github.com/Xilinx-CNS/kubernetes-onload/blob/master/pkg/deviceplugin/nic.go
//
// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: (c) Copyright 2023 Advanced Micro Devices, Inc.

type NICInfo struct {
	Interface string
	PCIBusID  string
}

// Returns a list of the Solarflare interfaces present on the node
func ProbeOnloadNics() ([]NICInfo, error) {
	// Takes the output from lshw and returns the device name for each Solarflare device.

	// "lshw -businfo -class network" sample output:
	// Bus info          Device     Class          Description
	// =======================================================
	// pci@0000:04:00.0  eth2       network        NetXtreme BCM5720 Gigabit Ethernet PCIe
	// pci@0000:04:00.1  eth3       network        NetXtreme BCM5720 Gigabit Ethernet PCIe
	// pci@0000:31:00.0             network        BCM57412 NetXtreme-E 10Gb RDMA Ethernet Controller
	// pci@0000:31:00.1             network        BCM57412 NetXtreme-E 10Gb RDMA Ethernet Controller
	// pci@0000:98:00.0  eth4       network        BCM57412 NetXtreme-E 10Gb RDMA Ethernet Controller
	// pci@0000:98:00.1  eth5       network        BCM57412 NetXtreme-E 10Gb RDMA Ethernet Controller
	// pci@0000:b1:00.0  eth0       network        SFC9220 10/40G Ethernet Controller
	// pci@0000:b1:00.1  eth1       network        SFC9220 10/40G Ethernet Controller

	// This regex makes the assumption that all interface names only
	// contain either lowercase letters or numbers. If that is not true,
	// then this should be updated to reflect that.
	// First match group is the PCI bus, second match group is the Interface
	r := regexp.MustCompile("^pci@([a-f0-9:.]+) *([a-z0-9]+) *network *.*SFC")

	cmdOutput, err := exec.Command("lshw", "-businfo", "-class", "network").CombinedOutput()
	if err != nil {
		return nil, err
	}

	var nics []NICInfo
	scanner := bufio.NewScanner(strings.NewReader(string(cmdOutput)))

	for scanner.Scan() {
		line := scanner.Text()
		m := r.FindStringSubmatch(line)
		if len(m) == 3 {
			iface, busid := m[2], m[1]
			nics = append(nics, NICInfo{iface, busid})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nics, nil
}
