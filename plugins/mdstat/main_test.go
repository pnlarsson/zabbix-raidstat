package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fixturesDir is the shared mock data, also used by testdata/run-tests.sh.
var fixturesDir = filepath.Join("..", "..", "testdata", "mdstat")

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir, name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

func TestParseArrayNames(t *testing.T) {
	got := parseArrayNames(readFixture(t, "scan.txt"))
	// md0/md1 are kernel names, md/2 is the named form, md/Volume0_0 is the IMSM
	// member volume; the md/imsm0 container (metadata=imsm) must be excluded.
	want := []string{"md0", "md1", "md/2", "md/Volume0_0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseArrayNames = %v, want %v", got, want)
	}
}

func TestParseArrayNamesSkipsContainers(t *testing.T) {
	scan := []byte(
		"ARRAY /dev/md0 metadata=1.2 UUID=a\n" +
			"ARRAY /dev/md/imsm0 metadata=imsm UUID=b\n" +
			"ARRAY /dev/md/ddfc metadata=ddf UUID=c\n" +
			"ARRAY /dev/md/Volume0_0 container=/dev/md/imsm0 member=0 UUID=d\n")
	got := parseArrayNames(scan)
	want := []string{"md0", "md/Volume0_0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseArrayNames = %v, want %v", got, want)
	}
}

func TestParseArrayNamesEmpty(t *testing.T) {
	if got := parseArrayNames(nil); len(got) != 0 {
		t.Errorf("parseArrayNames(nil) = %v, want empty", got)
	}
}

func TestParseMembers(t *testing.T) {
	cases := []struct {
		fixture string
		want    []string
	}{
		{"md0.txt", []string{"sda1", "sdb1"}},
		// md1 has an "active sync", a "removed" (no device), and a "faulty" row;
		// the removed row must be skipped.
		{"md1.txt", []string{"sdc1", "sdd1"}},
		// named array with NVMe members.
		{"md2.txt", []string{"nvme0n1p1", "nvme1n1p1"}},
		// IMSM volume: whole disks, reordered Number column.
		{"Volume0_0.txt", []string{"sda", "sdb"}},
	}
	for _, c := range cases {
		got := parseMembers(readFixture(t, c.fixture))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseMembers(%s) = %v, want %v", c.fixture, got, c.want)
		}
	}
}

func TestParseMembersIgnoresContainer(t *testing.T) {
	// An IMSM container lists disks with Number "-" and no State column;
	// none of those rows should be treated as members.
	if got := parseMembers(readFixture(t, "imsm0.txt")); len(got) != 0 {
		t.Errorf("parseMembers(imsm0) = %v, want empty", got)
	}
}

func TestParseArrayStateAndSize(t *testing.T) {
	cases := []struct {
		fixture   string
		wantState string
		wantSize  string
	}{
		{"md0.txt", "clean", "1047552 (1023.00 MiB 1072.69 MB)"},
		{"md1.txt", "clean, degraded", "2095104 (2046.00 MiB 2145.39 MB)"},
		{"Volume0_0.txt", "active", "222715904 (212.40 GiB 228.06 GB)"},
	}
	for _, c := range cases {
		detail := readFixture(t, c.fixture)
		if got := parseArrayState(detail); got != c.wantState {
			t.Errorf("parseArrayState(%s) = %q, want %q", c.fixture, got, c.wantState)
		}
		if got := parseArraySize(detail); got != c.wantSize {
			t.Errorf("parseArraySize(%s) = %q, want %q", c.fixture, got, c.wantSize)
		}
	}
}

func TestParseMemberState(t *testing.T) {
	md1 := readFixture(t, "md1.txt")
	if got := parseMemberState(md1, "sdc1"); got != "active sync" {
		t.Errorf("parseMemberState(md1, sdc1) = %q, want %q", got, "active sync")
	}
	if got := parseMemberState(md1, "sdd1"); got != "faulty" {
		t.Errorf("parseMemberState(md1, sdd1) = %q, want %q", got, "faulty")
	}
	// A device not present in the table yields "".
	if got := parseMemberState(md1, "sdz9"); got != "" {
		t.Errorf("parseMemberState(md1, sdz9) = %q, want empty", got)
	}
}

func TestIsStateBad(t *testing.T) {
	bad := []string{"clean, degraded", "active, degraded", "FAILED", "active, FAILED", "clean, Not Started"}
	for _, s := range bad {
		if !isStateBad(s) {
			t.Errorf("isStateBad(%q) = false, want true", s)
		}
	}
	good := []string{"clean", "active", "clean, resyncing", ""}
	for _, s := range good {
		if isStateBad(s) {
			t.Errorf("isStateBad(%q) = true, want false", s)
		}
	}
}

func TestNormalizeArrayStatus(t *testing.T) {
	cases := map[string]string{
		"clean":           "OK",
		"active":          "OK",
		"clean, degraded": "clean, degraded",
		"FAILED":          "FAILED",
	}
	for in, want := range cases {
		if got := normalizeArrayStatus(in); got != want {
			t.Errorf("normalizeArrayStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeMemberState(t *testing.T) {
	cases := map[string]string{
		"active sync":              "OK",
		"active sync, writemostly": "OK",
		"spare":                    "OK",
		"":                         "removed",
		"faulty":                   "faulty",
		"spare rebuilding":         "spare rebuilding",
	}
	for in, want := range cases {
		if got := normalizeMemberState(in); got != want {
			t.Errorf("normalizeMemberState(%q) = %q, want %q", in, got, want)
		}
	}
}
