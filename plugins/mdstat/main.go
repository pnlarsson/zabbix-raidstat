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
//
// The exported functions fetch mdadm output and delegate to the pure parse*/
// normalize* helpers below, which are covered by main_test.go.

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

// parseArrayNames - md array names (without the /dev/ prefix) from `--detail --scan`.
//
// External-metadata containers (Intel IMSM, DDF) are skipped: they only group
// disks and carry no array State/size of their own. Their member volumes are
// listed separately (with "container=") and report the real health.
func parseArrayNames(scan []byte) []string {
	arrays := []string{}
	for _, m := range arrayLine.FindAllStringSubmatch(string(scan), -1) {
		dev, attrs := m[1], m[2]
		if strings.Contains(attrs, "metadata=imsm") || strings.Contains(attrs, "metadata=ddf") {
			continue
		}
		arrays = append(arrays, strings.TrimPrefix(dev, "/dev/"))
	}
	return arrays
}

// parseMembers - component device names from a single array's `--detail` output.
func parseMembers(detail []byte) []string {
	members := []string{}
	for _, m := range deviceLine.FindAllStringSubmatch(string(detail), -1) {
		members = append(members, m[2])
	}
	return members
}

// parseArrayState - the array "State : ..." value.
func parseArrayState(detail []byte) string {
	return functions.TrimSpacesLeftAndRight(functions.GetRegexpSubmatch(detail, "State :(.*)"))
}

// parseArraySize - the "Array Size : ..." value.
func parseArraySize(detail []byte) string {
	return functions.TrimSpacesLeftAndRight(functions.GetRegexpSubmatch(detail, "Array Size :(.*)"))
}

// parseMemberState - the state of a single component device within an array.
func parseMemberState(detail []byte, device string) string {
	re := fmt.Sprintf("(?m)^\\s*\\d+\\s+\\d+\\s+\\d+\\s+\\S+\\s+(.+?)\\s+/dev/%s\\s*$", regexp.QuoteMeta(device))
	return functions.TrimSpacesLeftAndRight(functions.GetRegexpSubmatch(detail, re))
}

// isStateBad - whether an md array State indicates a problem.
func isStateBad(state string) bool {
	for _, bad := range []string{"degraded", "FAILED", "Not Started"} {
		if strings.Contains(state, bad) {
			return true
		}
	}
	return false
}

// normalizeArrayStatus - map a healthy array State to "OK", else pass it through.
func normalizeArrayStatus(state string) string {
	if isStateBad(state) {
		return state
	}
	return "OK"
}

// normalizeMemberState - map a component device state to a Zabbix-friendly value.
func normalizeMemberState(state string) string {
	switch {
	case strings.HasPrefix(state, "active sync"), state == "spare":
		return "OK"
	case state == "":
		// not found in the member table - the device was removed/missing
		return "removed"
	default:
		return state
	}
}

// getArrays - md array names from a live mdadm.
func getArrays(execPath string) []string {
	return parseArrayNames(functions.GetCommandOutput(execPath, "--detail", "--scan"))
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
		members := parseMembers(functions.GetCommandOutput(execPath, "--detail", "/dev/"+array))

		if os.Getenv("RAIDSTAT_DEBUG") == "y" {
			fmt.Printf("Regexp is '%s'\nMembers of %s: %v\n", deviceLine.String(), array, members)
		}

		for _, device := range members {
			data = append(data, fmt.Sprintf("%s:%s", array, device))
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
		state := parseArrayState(functions.GetCommandOutput(execPath, "--detail", "/dev/"+array))
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

	detail := functions.GetCommandOutput(execPath, "--detail", "/dev/"+deviceID)

	data := ReturnData{
		Status: normalizeArrayStatus(parseArrayState(detail)),
		Size:   parseArraySize(detail),
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

	detail := functions.GetCommandOutput(execPath, "--detail", "/dev/"+array)

	data := ReturnData{
		Status:    normalizeMemberState(parseMemberState(detail, device)),
		Model:     device,
		Smart:     "OK", // md exposes no SMART data; report OK so the SMART trigger stays quiet
		SmartWarn: "0",
	}

	return append(functions.MarshallJSON(data, indent), "\n"...)
}

func main() {}
