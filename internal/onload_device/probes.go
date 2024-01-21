// nomad-onload
// Copyright (c) 2024 Neomantra BV

package onload_device

import (
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

// Returns a list of the Solarflare interfaces present on the node
func ProbeOnloadNics() ([]string, error) {
	// Depending on what information we are looking for in the output I think it
	// is quite tempting to retrieve the information in a json format, then
	// parse this using golang's built-in features.
	bytes, err := exec.Command("lshw", "-short", "-class", "network").CombinedOutput()
	output := string(bytes)
	if err != nil {
		return nil, err
	}
	interfaces := parseOutput(output)
	return interfaces, nil
}

// Takes the output from lshw and returns the device name for each Solarflare device.
func parseOutput(output string) []string {
	// "lshw -short -class network" sample output:
	// H/W path            Device     Class          Description
	// =========================================================
	// /0/100/1b/0         enp2s0f0   network        XtremeScale SFC9250 10/25/40/50/100G Ethernet Controller
	// /0/100/1b/0.1       enp2s0f1   network        XtremeScale SFC9250 10/25/40/50/100G Ethernet Controller
	// /0/100/1c.1/0       eno1       network        NetXtreme BCM5720 Gigabit Ethernet PCIe
	// /0/100/1c.1/0.1     eno2       network        NetXtreme BCM5720 Gigabit Ethernet PCIe

	lines := strings.Split(output, "\n")
	var interfaces []string

	// Assume that we are running as root, if not then we would have to skip
	// an additional line at the start of the output
	skip_lines := 2
	end_lines := 1
	if os.Geteuid() != 0 {
		skip_lines = 3
		end_lines = 2
	}

	for _, line := range lines[skip_lines : len(lines)-end_lines] {
		// This regex makes the assumption that all interface names only
		// contain either lowercase letters or numbers. If that is not true,
		// then this should be updated to reflect that.
		r := regexp.MustCompile("([a-z0-9]+) *network *.*SFC")
		out := r.FindStringSubmatch(line)
		if out != nil {
			// It is safe to access out[1] here since the return value of
			// FindStringSubmatch is an array where the first value is the
			// whole string and any subsequent values are the submatches.
			// In this case since there is a submatch that should match the
			// device name if FindStringSubmatch returns non-nil then there
			// will be at least 2 elements in the return array.
			interfaces = append(interfaces, out[1])
		}
	}
	return interfaces
}
