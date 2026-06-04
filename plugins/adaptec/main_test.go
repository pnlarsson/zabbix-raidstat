package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// These are integration tests: they run the exported functions against the
// mock arcconf in testdata/adaptec.sh, which cats canned output from
// testdata/adaptec/*.txt. The mock uses paths relative to the repo root.

const mock = "testdata/adaptec.sh"

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
	wantIDs(t, "controllers", GetControllersIDs(mock), []string{"1"})
	wantIDs(t, "logicaldrives", GetLogicalDrivesIDs(mock, "1"), []string{"0"})
	wantIDs(t, "physicaldrives", GetPhysicalDrivesIDs(mock, "1"),
		[]string{"0,0", "0,1", "0,2", "0,3", "0,4", "0,5", "0,6", "0,7"})
}

func TestControllerStatus(t *testing.T) {
	s := status(t, GetControllerStatus(mock, "1", 0))
	if s["status"] != "OK" || s["model"] != "Adaptec 6805" {
		t.Errorf("controller status = %v", s)
	}
}

func TestLogicalDriveStatus(t *testing.T) {
	s := status(t, GetLDStatus(mock, "1", "0", 0))
	if s["status"] != "OK" || s["size"] != "7618550 MB" {
		t.Errorf("ld status = %v", s)
	}
}

func TestPhysicalDriveStatus(t *testing.T) {
	s := status(t, GetPDStatus(mock, "1", "0,0", 0))
	if s["status"] != "OK" || s["smart"] != "OK" {
		t.Errorf("pd status = %v", s)
	}
	if s["model"] == "" {
		t.Errorf("pd model is empty: %v", s)
	}
}
