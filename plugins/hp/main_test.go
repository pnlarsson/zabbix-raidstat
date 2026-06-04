package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Integration tests against the mock ssacli in testdata/hp.sh.

const mock = "testdata/hp.sh"

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
	wantIDs(t, "logicaldrives", GetLogicalDrivesIDs(mock, "0"), []string{"1", "2"})
	wantIDs(t, "physicaldrives", GetPhysicalDrivesIDs(mock, "0"),
		[]string{"1I:1:1", "1I:1:2", "1I:1:3", "1I:1:4", "2I:1:5", "2I:1:6", "2I:1:7", "2I:1:8"})
}

func TestControllerStatus(t *testing.T) {
	s := status(t, GetControllerStatus(mock, "0", 0))
	if s["status"] != "OK" || s["model"] != "Smart Array P410i" {
		t.Errorf("controller status = %v", s)
	}
	if s["batterystatus"] != "OK" {
		t.Errorf("batterystatus = %q, want OK", s["batterystatus"])
	}
	if s["cachestatus"] == "" {
		t.Errorf("cachestatus is empty: %v", s)
	}
}

func TestLogicalDriveStatus(t *testing.T) {
	s := status(t, GetLDStatus(mock, "0", "1", 0))
	if s["status"] != "OK" || s["size"] != "558.9 GB" {
		t.Errorf("ld status = %v", s)
	}
}

func TestPhysicalDriveStatus(t *testing.T) {
	s := status(t, GetPDStatus(mock, "0", "1I:1:1", 0))
	if s["status"] != "OK" || s["size"] != "600 GB" {
		t.Errorf("pd status = %v", s)
	}
	if s["model"] == "" {
		t.Errorf("pd model is empty: %v", s)
	}
}
