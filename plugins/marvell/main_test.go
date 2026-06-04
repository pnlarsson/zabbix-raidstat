package main

import (
	"os"
	"reflect"
	"testing"
)

// Integration tests against the mock mvcli in testdata/marvell.sh.
// The mock only provides discovery output (no per-device status fixtures),
// so only the discovery functions are exercised here.

const mock = "testdata/marvell.sh"

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

func TestDiscovery(t *testing.T) {
	wantIDs(t, "controllers", GetControllersIDs(mock), []string{"0"})
	wantIDs(t, "logicaldrives", GetLogicalDrivesIDs(mock, "0"), []string{"0"})
	wantIDs(t, "physicaldrives", GetPhysicalDrivesIDs(mock, "0"), []string{"0", "1"})
}
