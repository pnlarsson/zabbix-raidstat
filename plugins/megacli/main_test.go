package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Integration tests against the mock megacli in testdata/megacli.sh.

const mock = "testdata/megacli.sh"

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil { // repo root
		panic(err)
	}
	_ = os.Chmod(mock, 0o755)
	os.Exit(m.Run())
}

func wantIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func status(t *testing.T, raw []byte) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("status JSON %q: %v", raw, err)
	}
	return m
}

func TestDiscovery(t *testing.T) {
	wantIDs(t, "controllers", GetControllersIDs(mock), []string{"0"})
	wantIDs(t, "logicaldrives", GetLogicalDrivesIDs(mock, "0"), []string{"2"})
	wantIDs(t, "physicaldrives", GetPhysicalDrivesIDs(mock, "0"),
		[]string{"252:0", "252:1", "252:2", "252:3", "252:4", "252:5"})
}

// TestControllerStatus also guards the BBU fix: a healthy "Battery State:
// Optimal" must normalize to "OK" (not leak "Optimal" or an empty string).
func TestControllerStatus(t *testing.T) {
	s := status(t, GetControllerStatus(mock, "0", 0))
	if s["status"] != "OK" || s["model"] != "LSI MegaRAID SAS 9261-8i" {
		t.Errorf("controller status = %v", s)
	}
	if s["batterystatus"] != "OK" {
		t.Errorf("batterystatus = %q, want OK", s["batterystatus"])
	}
}

func TestLogicalDriveStatus(t *testing.T) {
	s := status(t, GetLDStatus(mock, "0", "2", 0))
	if s["status"] != "OK" || s["size"] != "893.137 GB" {
		t.Errorf("ld status = %v", s)
	}
}

func TestPhysicalDriveStatus(t *testing.T) {
	s := status(t, GetPDStatus(mock, "0", "252:0", 0))
	if s["status"] != "OK" || s["smart"] != "OK" {
		t.Errorf("pd status = %v", s)
	}
	if s["model"] == "" {
		t.Errorf("pd model is empty: %v", s)
	}
}
