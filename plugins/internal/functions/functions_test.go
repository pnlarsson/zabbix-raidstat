package functions

import (
	"reflect"
	"testing"
)

func TestTrimSpacesLeftAndRight(t *testing.T) {
	cases := map[string]string{
		"  hello  ":     "hello",
		"no-pad":        "no-pad",
		"   leading":    "leading",
		"trailing   ":   "trailing",
		"  in between ": "in between",
		"":              "",
	}
	for in, want := range cases {
		if got := TrimSpacesLeftAndRight(in); got != want {
			t.Errorf("TrimSpacesLeftAndRight(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetRegexpSubmatch(t *testing.T) {
	buf := []byte("Status of Logical Device : Optimal\nSize : 100 GB\n")

	if got := GetRegexpSubmatch(buf, "Status of Logical Device : (.*)"); got != "Optimal" {
		t.Errorf("submatch = %q, want %q", got, "Optimal")
	}
	// No match returns an empty string rather than panicking.
	if got := GetRegexpSubmatch(buf, "Nonexistent : (.*)"); got != "" {
		t.Errorf("submatch (no match) = %q, want empty", got)
	}
}

func TestGetRegexpAllSubmatch(t *testing.T) {
	buf := []byte("Controller 0:\nController 1:\nController 2:\n")

	got := GetRegexpAllSubmatch(buf, `Controller (\d+):`)
	want := []string{"0", "1", "2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("allsubmatch = %v, want %v", got, want)
	}

	if got := GetRegexpAllSubmatch(buf, `nope(\d+)`); len(got) != 0 {
		t.Errorf("allsubmatch (no match) = %v, want empty", got)
	}
}

func TestMarshallJSON(t *testing.T) {
	data := struct {
		Status string `json:"status"`
	}{Status: "OK"}

	if got := string(MarshallJSON(data, 0)); got != `{"status":"OK"}` {
		t.Errorf("MarshallJSON(indent=0) = %q", got)
	}

	want := "{\n  \"status\": \"OK\"\n}"
	if got := string(MarshallJSON(data, 2)); got != want {
		t.Errorf("MarshallJSON(indent=2) = %q, want %q", got, want)
	}
}

// TestGetCommandOutputAllowExit covers the tolerance that the megacli BBU fix
// relies on: a tool that writes output and then exits non-zero (mdadm/megacli
// do this when a BBU/array is missing) must still have its stdout returned.
func TestGetCommandOutputAllowExit(t *testing.T) {
	got := GetCommandOutputAllowExit("sh", "-c", "printf 'partial output'; exit 34")
	if string(got) != "partial output" {
		t.Errorf("GetCommandOutputAllowExit = %q, want %q", string(got), "partial output")
	}
}

func TestGetCommandOutput(t *testing.T) {
	got := GetCommandOutput("sh", "-c", "printf 'ok'")
	if string(got) != "ok" {
		t.Errorf("GetCommandOutput = %q, want %q", string(got), "ok")
	}
}
