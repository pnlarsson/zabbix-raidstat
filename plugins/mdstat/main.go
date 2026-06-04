package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ps78674/zabbix-raidstat/plugins/internal/functions"
)

// mdstat - Linux software RAID (md) support via mdadm.
//
// There is no hardware controller, so a single pseudo-controller "0" is
// reported whenever any md array exists. Each md array is exposed as a logical
// drive (LD_ID = "md0", "md1", ...) and each component block device as a
// physical drive (PD_ID = "<array>:<device>", e.g. "md0:sda1").

// deviceLine matches a member row of `mdadm --detail` output, capturing the
// state (group 1) and the device name (group 2):
//
//	0       8        1        0      active sync   /dev/sda1
//
// Rows without a /dev path (e.g. "removed") are intentionally not matched.
var deviceLine = regexp.MustCompile(`(?m)^\s*\d+\s+\d+\s+\d+\s+\S+\s+(.+?)\s+/dev/(\S+)\s*$`)

// arrayLine matches an "ARRAY <device> <attrs...>" row of `mdadm --detail --scan`,
// capturing the device path (group 1) and the trailing attributes (group 2).
var arrayLine = regexp.MustCompile(`(?m)^ARRAY\s+(\S+)(.*)$`)

// getArrays - return md array names (without the /dev/ prefix), e.g. "md0".
//
// External-metadata containers (Intel IMSM, DDF) are skipped: they only group
// disks and carry no array State/size of their own. Their member volumes are
// listed separately (with "container=") and report the real health.
func getArrays(execPath string) []string {
	inputData := functions.GetCommandOutput(execPath, "--detail", "--scan")

	arrays := []string{}
	for _, m := range arrayLine.FindAllStringSubmatch(string(inputData), -1) {
		dev, attrs := m[1], m[2]
		if strings.Contains(attrs, "metadata=imsm") || strings.Contains(attrs, "metadata=ddf") {
			continue
		}
		arrays = append(arrays, strings.TrimPrefix(dev, "/dev/"))
	}

	return arrays
}

// isStateBad - report whether an md array State indicates a problem.
func isStateBad(state string) bool {
	for _, bad := range []string{"degraded", "FAILED", "Not Started"} {
		if strings.Contains(state, bad) {
			return true
		}
	}
	return false
}

// GetControllersIDs - md has no controllers; report a single pseudo-controller
// "0" only when at least one array exists.
func GetControllersIDs(execPath string) []string {
	if len(getArrays(execPath)) == 0 {
		return []string{}
	}
	return []string{"0"}
}

// GetLogicalDrivesIDs - each md array is a logical drive.
func GetLogicalDrivesIDs(execPath string, controllerID string) []string {
	return getArrays(execPath)
}

// GetPhysicalDrivesIDs - each component device, keyed as "<array>:<device>".
func GetPhysicalDrivesIDs(execPath string, controllerID string) []string {
	data := []string{}

	for _, array := range getArrays(execPath) {
		inputData := functions.GetCommandOutput(execPath, "--detail", "/dev/"+array)

		result := deviceLine.FindAllStringSubmatch(string(inputData), -1)

		if os.Getenv("RAIDSTAT_DEBUG") == "y" {
			fmt.Printf("Regexp is '%s'\n", deviceLine.String())
			fmt.Printf("Result is '%s'\n", result)
		}

		for _, v := range result {
			data = append(data, fmt.Sprintf("%s:%s", array, v[2]))
		}
	}

	return data
}

// GetControllerStatus - aggregate state across all arrays.
func GetControllerStatus(execPath string, controllerID string, indent int) []byte {
	type ReturnData struct {
		Status string `json:"status"`
		Model  string `json:"model"`
	}

	problems := []string{}
	for _, array := range getArrays(execPath) {
		inputData := functions.GetCommandOutput(execPath, "--detail", "/dev/"+array)
		state := functions.TrimSpacesLeftAndRight(functions.GetRegexpSubmatch(inputData, "State :(.*)"))
		if isStateBad(state) {
			problems = append(problems, fmt.Sprintf("%s is %s", array, state))
		}
	}

	status := "OK"
	if len(problems) > 0 {
		status = strings.Join(problems, ", ")
	}

	data := ReturnData{
		Status: status,
		Model:  "Linux Software RAID (md)",
	}

	return append(functions.MarshallJSON(data, indent), "\n"...)
}

// GetLDStatus - status and size of a single array.
func GetLDStatus(execPath string, controllerID string, deviceID string, indent int) []byte {
	type ReturnData struct {
		Status string `json:"status"`
		Size   string `json:"size"`
	}

	inputData := functions.GetCommandOutput(execPath, "--detail", "/dev/"+deviceID)
	state := functions.TrimSpacesLeftAndRight(functions.GetRegexpSubmatch(inputData, "State :(.*)"))
	size := functions.TrimSpacesLeftAndRight(functions.GetRegexpSubmatch(inputData, "Array Size :(.*)"))

	status := state
	if !isStateBad(state) {
		status = "OK"
	}

	data := ReturnData{
		Status: status,
		Size:   size,
	}

	return append(functions.MarshallJSON(data, indent), "\n"...)
}

// GetPDStatus - status of a single component device.
func GetPDStatus(execPath string, controllerID string, deviceID string, indent int) []byte {
	type ReturnData struct {
		Status    string `json:"status"`
		Model     string `json:"model"`
		Smart     string `json:"smart"`
		SmartWarn string `json:"smartwarnings"`
	}

	parts := strings.SplitN(deviceID, ":", 2)
	if len(parts) != 2 {
		fmt.Printf("Error - wrong device id '%s'.\n", deviceID)
		os.Exit(1)
	}
	array, device := parts[0], parts[1]

	inputData := functions.GetCommandOutput(execPath, "--detail", "/dev/"+array)
	re := fmt.Sprintf("(?m)^\\s*\\d+\\s+\\d+\\s+\\d+\\s+\\S+\\s+(.+?)\\s+/dev/%s\\s*$", regexp.QuoteMeta(device))
	state := functions.TrimSpacesLeftAndRight(functions.GetRegexpSubmatch(inputData, re))

	var status string
	switch {
	case strings.HasPrefix(state, "active sync"), state == "spare":
		status = "OK"
	case state == "":
		// not found in the member table - the device was removed/missing
		status = "removed"
	default:
		status = state
	}

	data := ReturnData{
		Status:    status,
		Model:     device,
		Smart:     "OK", // md exposes no SMART data; report OK so the SMART trigger stays quiet
		SmartWarn: "0",
	}

	return append(functions.MarshallJSON(data, indent), "\n"...)
}

func main() {}
