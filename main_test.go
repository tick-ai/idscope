package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "idscope-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "idscope")
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build failed: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type result struct {
	out, err string
	code     int
}

// idscope runs the built binary as a child process in an empty folder.
func idscope(t *testing.T, args ...string) result {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = t.TempDir()
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("running idscope %q: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return result{so.String(), se.String(), code}
}

// block parses "key: value" lines into ordered keys and a map.
func block(t *testing.T, text string) ([]string, map[string]string) {
	t.Helper()
	var keys []string
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			t.Fatalf("line %q is not key: value", line)
		}
		keys = append(keys, k)
		m[k] = v
	}
	return keys, m
}

// inspectOne inspects one id in the plain form and returns its block.
func inspectOne(t *testing.T, in string) ([]string, map[string]string, int) {
	t.Helper()
	r := idscope(t, "inspect", in)
	if strings.Contains(in, "\n") {
		// the plain block of an id with a newline in it is not line-parsable: read the JSON
		var v []map[string]any
		if err := json.Unmarshal([]byte(idscope(t, "inspect", "--json", in).out), &v); err != nil || len(v) != 1 {
			t.Fatalf("json for %q: %v", in, err)
		}
		m := map[string]string{}
		for k, x := range v[0] {
			m[k] = fmt.Sprint(x)
		}
		return nil, m, r.code
	}
	keys, m := block(t, r.out)
	return keys, m, r.code
}

func inspectJSON(t *testing.T, ids ...string) []map[string]any {
	t.Helper()
	r := idscope(t, append([]string{"inspect", "--json"}, ids...)...)
	if !strings.HasSuffix(r.out, "\n") || strings.Count(r.out, "\n") != 1 {
		t.Fatalf("json output is not one line: %q", r.out)
	}
	var v []map[string]any
	if err := json.Unmarshal([]byte(r.out), &v); err != nil {
		t.Fatalf("bad json %q: %v", r.out, err)
	}
	return v
}

func TestInspectJobExample(t *testing.T) {
	r := idscope(t, "inspect", "017F22E2-79B0-7CC3-98C4-DC0C0C07398F", "01ARZ3NDEKTSV4RRFFQ69G5FAV", "nope")
	want := `input: 017F22E2-79B0-7CC3-98C4-DC0C0C07398F
valid: true
kind: uuid
canonical: 017f22e2-79b0-7cc3-98c4-dc0c0c07398f
variant: rfc9562
version: 7
timestamp: 2022-02-22T19:22:22.000Z

input: 01ARZ3NDEKTSV4RRFFQ69G5FAV
valid: true
kind: ulid
canonical: 01ARZ3NDEKTSV4RRFFQ69G5FAV
timestamp: 2016-07-30T23:54:10.259Z

input: nope
valid: false
error: unrecognised
`
	if r.out != want || r.code != 1 || r.err != "" {
		t.Fatalf("got code %d, stderr %q, stdout:\n%s\nwant:\n%s", r.code, r.err, r.out, want)
	}
}

func TestInspectOrderAndRepeats(t *testing.T) {
	r := idscope(t, "inspect", "nope", "00000000-0000-0000-0000-000000000000", "nope")
	want := "input: nope\nvalid: false\nerror: unrecognised\n\n" +
		"input: 00000000-0000-0000-0000-000000000000\nvalid: true\nkind: uuid\ncanonical: 00000000-0000-0000-0000-000000000000\nvariant: nil\n\n" +
		"input: nope\nvalid: false\nerror: unrecognised\n"
	if r.out != want || r.code != 1 {
		t.Fatalf("got %d %q", r.code, r.out)
	}
	r = idscope(t, "inspect", "")
	if r.out != "input: \nvalid: false\nerror: unrecognised\n" || r.code != 1 {
		t.Fatalf("empty argument: %d %q", r.code, r.out)
	}
	r = idscope(t, "inspect", "00000000-0000-0000-0000-000000000000")
	if r.code != 0 {
		t.Fatalf("valid id exit %d", r.code)
	}
}

func TestUUIDForms(t *testing.T) {
	const base = "017f22e2-79b0-7cc3-98c4-dc0c0c07398f"
	const hex32 = "017f22e279b07cc398c4dc0c0c07398f"
	valid := []string{
		base,
		strings.ToUpper(base),
		"017F22e2-79B0-7cc3-98C4-dc0c0C07398f",
		hex32,
		strings.ToUpper(hex32),
		"{" + base + "}",
		"{" + strings.ToUpper(base) + "}",
		"urn:uuid:" + base,
		"URN:UUID:" + strings.ToUpper(base),
		"Urn:Uuid:" + base,
		"uRn:UuId:" + base,
	}
	for _, in := range valid {
		_, m, code := inspectOne(t, in)
		if code != 0 || m["valid"] != "true" || m["kind"] != "uuid" || m["canonical"] != base || m["input"] != in {
			t.Errorf("%q: code %d block %v", in, code, m)
		}
	}
	invalid := []string{
		"{" + hex32 + "}",
		"urn:uuid:" + hex32,
		"{urn:uuid:" + base + "}",
		"urn:uuid:{" + base + "}",
		"{" + base,
		base + "}",
		"{" + base[:35] + "}",
		" " + base,
		base + " ",
		base + "\n",
		"\t" + hex32,
		base[:35],
		base + "0",
		hex32 + "0",
		hex32[:31],
		"017f22e279b0-7cc3-98c4-dc0c0c07398f",
		"017f22e2-79b07-cc3-98c4-dc0c0c07398f",
		"017f22e2-79b0-7cc3-98c4dc0c0c07398f-",
		"_" + base[:35],
		"017f22e2_79b0_7cc3_98c4_dc0c0c07398f",
		"g17f22e2-79b0-7cc3-98c4-dc0c0c07398f",
		"017f22e2-79b0-7cc3-98c4-dc0c0c07398g",
		"urn:uuid" + base,
		"urn-uuid:" + base,
		"urn:uuid:" + base + "0",
		"urn:guid:" + base,
		"0x" + hex32,
		"nope",
		"",
		"017f22e2-79b0-7cc3-98c4-dc0c0c07398é",
	}
	for _, in := range invalid {
		_, m, code := inspectOne(t, in)
		if code != 1 || m["valid"] != "false" || m["error"] != "unrecognised" || m["input"] != in {
			t.Errorf("%q: code %d block %v", in, code, m)
		}
	}
}

func TestULIDRules(t *testing.T) {
	const canon = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	ok := map[string]string{
		canon:                               canon,
		strings.ToLower(canon):              canon,
		"01arZ3ndEKtsv4rrFFq69g5fAV":        canon,
		"O1ARZ3NDEKTSV4RRFFQ69G5FAV":        canon,
		"o1ARZ3NDEKTSV4RRFFQ69G5FAV":        canon,
		"0IARZ3NDEKTSV4RRFFQ69G5FAV":        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"0lARZ3NDEKTSV4RRFFQ69G5FAV":        "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"0LARZ3NDEKTSV4RRFFQ69G5FAO":        "01ARZ3NDEKTSV4RRFFQ69G5FA0",
		"I1ARZ3NDEKTSV4RRFFQ69G5FAV":        "11ARZ3NDEKTSV4RRFFQ69G5FAV",
		"l1ARZ3NDEKTSV4RRFFQ69G5FAV":        "11ARZ3NDEKTSV4RRFFQ69G5FAV",
		"OOOOOOOOOOOOOOOOOOOOOOOOOO":        "00000000000000000000000000",
		"7ZZZZZZZZZZZZZZZZZZZZZZZZZ":        "7ZZZZZZZZZZZZZZZZZZZZZZZZZ",
		"7zzzzzzzzzzzzzzzzzzzzzzzzz":        "7ZZZZZZZZZZZZZZZZZZZZZZZZZ",
		"IIIIIIIIIIIIIIIIIIIIIIIIII":        "11111111111111111111111111",
		"0000000000000000000000000L":        "00000000000000000000000001",
		"70000000000000000000000000":        "70000000000000000000000000",
		"00000000000000000000000000":        "00000000000000000000000000",
		"0123456789ABCDEFGHJKMNPQRS":        "0123456789ABCDEFGHJKMNPQRS",
		"0123456789abcdefghjkmnpqrs":        "0123456789ABCDEFGHJKMNPQRS",
		"1TVWXYZ" + strings.Repeat("0", 19): "1TVWXYZ" + strings.Repeat("0", 19),
	}
	for in, want := range ok {
		_, m, code := inspectOne(t, in)
		if code != 0 || m["valid"] != "true" || m["kind"] != "ulid" || m["canonical"] != want || m["input"] != in {
			t.Errorf("%q: code %d block %v, want canonical %s", in, code, m, want)
		}
	}
	bad := map[string]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAU":     "ulid-bad-character",
		"01ARZ3NDEKTSV4RRFFQ69G5FAu":     "ulid-bad-character",
		"U1ARZ3NDEKTSV4RRFFQ69G5FAV":     "ulid-bad-character",
		"01ARZ3NDEKTSV4RRFFQ69G5FA-":     "ulid-bad-character",
		"01ARZ3ND-KTSV4RRFFQ69G5FAV":     "ulid-bad-character",
		"01ARZ3NDEKTSV4RRFFQ69G5FA ":     "ulid-bad-character",
		"01ARZ3NDEKTSV4RRFFQ69G5FA=":     "ulid-bad-character",
		"01ARZ3NDEKTSV4RRFFQ69G5FA\n":    "ulid-bad-character",
		canon[:24] + "é":                 "ulid-bad-character",
		"8000000000000000000000000U":     "ulid-bad-character",
		"Z000000000000000000000000U":     "ulid-bad-character",
		"80000000000000000000000000":     "ulid-overflow",
		"8ZZZZZZZZZZZZZZZZZZZZZZZZZ":     "ulid-overflow",
		"90000000000000000000000000":     "ulid-overflow",
		"A0000000000000000000000000":     "ulid-overflow",
		"a0000000000000000000000000":     "ulid-overflow",
		"Z0000000000000000000000000":     "ulid-overflow",
		"ZZZZZZZZZZZZZZZZZZZZZZZZZZ":     "ulid-overflow",
		"01ARZ3NDEKTSV4RRFFQ69G5FA":      "unrecognised",
		"01ARZ3NDEKTSV4RRFFQ69G5FAVX":    "unrecognised",
		"01ARZ3NDEKTSV4RRFFQ69G5FAV0000": "unrecognised",
		"0":                              "unrecognised",
		"  01ARZ3NDEKTSV4RRFFQ69G5FAV  ": "unrecognised",
		"01ARZ3NDEKTSV4RRFFQ69G5FAV\n":   "unrecognised",
		"01ARZ3NDEK-TSV4RRFFQ69G5FAV":    "unrecognised",
		"01ARZ3ND-EKTSV4RRFFQ69G5FAV0":   "unrecognised",
	}
	for in, want := range bad {
		_, m, code := inspectOne(t, in)
		if code != 1 || m["valid"] != "false" || m["error"] != want {
			t.Errorf("%q: code %d block %v, want %s", in, code, m, want)
		}
	}
}

func TestVariants(t *testing.T) {
	cases := []struct{ in, variant string }{
		{"00000000-0000-0000-0000-000000000000", "nil"},
		{"ffffffff-ffff-ffff-ffff-ffffffffffff", "max"},
		{"FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF", "max"},
		{"00000000-0000-0000-0000-000000000001", "ncs"},
		{"00000000-0000-0000-7000-000000000000", "ncs"},
		{"7fffffff-ffff-ffff-7fff-ffffffffffff", "ncs"},
		{"00000000-0000-0000-8000-000000000000", "rfc9562"},
		{"00000000-0000-0000-9000-000000000000", "rfc9562"},
		{"00000000-0000-0000-a000-000000000000", "rfc9562"},
		{"00000000-0000-0000-bfff-ffffffffffff", "rfc9562"},
		{"00000000-0000-0000-c000-000000000000", "microsoft"},
		{"00000000-0000-0000-dfff-ffffffffffff", "microsoft"},
		{"00000000-0000-0000-e000-000000000000", "future"},
		{"00000000-0000-0000-f000-000000000000", "future"},
		{"ffffffff-ffff-ffff-ffff-fffffffffffe", "future"},
		{"fffffffe-ffff-ffff-ffff-ffffffffffff", "future"},
	}
	for _, c := range cases {
		keys, m, code := inspectOne(t, c.in)
		if code != 0 || m["variant"] != c.variant {
			t.Errorf("%s: variant %q code %d", c.in, m["variant"], code)
		}
		if c.variant != "rfc9562" {
			// no version, and so no fields that come from it
			want := []string{"input", "valid", "kind", "canonical", "variant"}
			if strings.Join(keys, ",") != strings.Join(want, ",") {
				t.Errorf("%s: keys %v", c.in, keys)
			}
		}
	}
	// a version digit under another variant is not read
	for _, in := range []string{"00000000-0000-1000-0000-000000000000", "ffffffff-ffff-7fff-ffff-ffffffffffff", "00000000-0000-6000-c000-000000000000"} {
		_, m, _ := inspectOne(t, in)
		for _, k := range []string{"version", "timestamp", "clock_seq", "node"} {
			if _, has := m[k]; has {
				t.Errorf("%s: has %s", in, k)
			}
		}
	}
}

func TestVersions(t *testing.T) {
	for v := 0; v < 16; v++ {
		in := fmt.Sprintf("00000000-0000-%x000-8000-000000000000", v)
		keys, m, code := inspectOne(t, in)
		if code != 0 || m["valid"] != "true" || m["variant"] != "rfc9562" || m["version"] != fmt.Sprint(v) {
			t.Errorf("%s: %v code %d", in, m, code)
		}
		want := "input,valid,kind,canonical,variant,version"
		switch v {
		case 1, 6:
			want += ",timestamp,clock_seq,node"
		case 7:
			want += ",timestamp"
		}
		if strings.Join(keys, ",") != want {
			t.Errorf("%s: keys %v, want %s", in, keys, want)
		}
	}
}

func TestTimestamps(t *testing.T) {
	cases := []struct {
		in, ts, seq, node string
	}{
		// RFC 9562 appendix A.1 and A.2: 2022-02-22 14:22:22 at UTC-5
		{"C232AB00-9414-11EC-B3C8-9E6BDECED846", "2022-02-22T19:22:22.0000000Z", "13256", "9e6bdeced846"},
		{"1EC9414C-232A-6B00-B3C8-9F6BDECED846", "2022-02-22T19:22:22.0000000Z", "13256", "9f6bdeced846"},
		// the origin and the largest values
		{"00000000-0000-1000-8000-000000000000", "1582-10-15T00:00:00.0000000Z", "0", "000000000000"},
		{"00000000-0000-6000-8000-000000000000", "1582-10-15T00:00:00.0000000Z", "0", "000000000000"},
		{"ffffffff-ffff-1fff-bfff-ffffffffffff", "5236-03-31T21:21:00.6846975Z", "16383", "ffffffffffff"},
		{"ffffffff-ffff-6fff-bfff-ffffffffffff", "5236-03-31T21:21:00.6846975Z", "16383", "ffffffffffff"},
		// the Unix epoch, and the last 100 ns before it
		{"13814000-1dd2-11b2-8000-000000000000", "1970-01-01T00:00:00.0000000Z", "0", "000000000000"},
		{"13813fff-1dd2-11b2-8000-000000000000", "1969-12-31T23:59:59.9999999Z", "0", "000000000000"},
		{"1b21dd21-3814-6000-8000-000000000000", "1970-01-01T00:00:00.0000000Z", "0", "000000000000"},
		{"1b21dd21-3813-6fff-8000-000000000000", "1969-12-31T23:59:59.9999999Z", "0", "000000000000"},
		// 1 and 10 ticks past the origin; fields in distinct places
		{"00000001-0000-1000-8000-000000000000", "1582-10-15T00:00:00.0000001Z", "0", "000000000000"},
		{"00000000-0000-6001-8001-000000000000", "1582-10-15T00:00:00.0000001Z", "1", "000000000000"},
		{"00000000-0001-1000-8123-0123456789ab", "1582-10-15T00:07:09.4967296Z", "291", "0123456789ab"},
		// version 7
		{"017F22E2-79B0-7CC3-98C4-DC0C0C07398F", "2022-02-22T19:22:22.000Z", "", ""},
		{"00000000-0000-7000-8000-000000000000", "1970-01-01T00:00:00.000Z", "", ""},
		{"00000000-0001-7000-8000-000000000000", "1970-01-01T00:00:00.001Z", "", ""},
		{"00000000-03e8-7fff-bfff-ffffffffffff", "1970-01-01T00:00:01.000Z", "", ""},
		{"ffffffff-ffff-7fff-bfff-ffffffffffff", "10889-08-02T05:31:50.655Z", "", ""},
		// ULID
		{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "2016-07-30T23:54:10.259Z", "", ""},
		{"00000000000000000000000000", "1970-01-01T00:00:00.000Z", "", ""},
		{"00000000010000000000000000", "1970-01-01T00:00:00.001Z", "", ""},
		{"7ZZZZZZZZZZZZZZZZZZZZZZZZZ", "10889-08-02T05:31:50.655Z", "", ""},
		{"01ARZ3NDEK0000000000000000", "2016-07-30T23:54:10.259Z", "", ""},
	}
	for _, c := range cases {
		_, m, code := inspectOne(t, c.in)
		if code != 0 || m["timestamp"] != c.ts {
			t.Errorf("%s: timestamp %q, want %s", c.in, m["timestamp"], c.ts)
		}
		if c.seq != "" && (m["clock_seq"] != c.seq || m["node"] != c.node) {
			t.Errorf("%s: clock_seq %q node %q, want %s %s", c.in, m["clock_seq"], m["node"], c.seq, c.node)
		}
	}
	// a v7 timestamp does not read the bits after the 48
	_, m, _ := inspectOne(t, "017f22e2-79b0-7cc3-98c4-dc0c0c07398f")
	if _, has := m["clock_seq"]; has {
		t.Error("v7 has clock_seq")
	}
}

func TestInspectJSON(t *testing.T) {
	v := inspectJSON(t, "C232AB00-9414-11EC-B3C8-9E6BDECED846", "01ARZ3NDEKTSV4RRFFQ69G5FAV", "nope", "00000000-0000-0000-0000-000000000000", "80000000000000000000000000", "01ARZ3NDEKTSV4RRFFQ69G5FAU")
	want := []map[string]any{
		{"input": "C232AB00-9414-11EC-B3C8-9E6BDECED846", "valid": true, "kind": "uuid", "canonical": "c232ab00-9414-11ec-b3c8-9e6bdeced846",
			"variant": "rfc9562", "version": 1.0, "timestamp": "2022-02-22T19:22:22.0000000Z", "clock_seq": 13256.0, "node": "9e6bdeced846"},
		{"input": "01ARZ3NDEKTSV4RRFFQ69G5FAV", "valid": true, "kind": "ulid", "canonical": "01ARZ3NDEKTSV4RRFFQ69G5FAV", "timestamp": "2016-07-30T23:54:10.259Z"},
		{"input": "nope", "valid": false, "error": "unrecognised"},
		{"input": "00000000-0000-0000-0000-000000000000", "valid": true, "kind": "uuid", "canonical": "00000000-0000-0000-0000-000000000000", "variant": "nil"},
		{"input": "80000000000000000000000000", "valid": false, "error": "ulid-overflow"},
		{"input": "01ARZ3NDEKTSV4RRFFQ69G5FAU", "valid": false, "error": "ulid-bad-character"},
	}
	if len(v) != len(want) {
		t.Fatalf("got %d elements: %v", len(v), v)
	}
	for i := range want {
		if fmt.Sprint(v[i]) != fmt.Sprint(want[i]) {
			t.Errorf("element %d:\n got %v\nwant %v", i, v[i], want[i])
		}
	}
	r := idscope(t, "inspect", "--json", "nope")
	if r.code != 1 {
		t.Errorf("json invalid exit %d", r.code)
	}
	r = idscope(t, "inspect", "", "--json")
	if r.out != "[{\"input\":\"\",\"valid\":false,\"error\":\"unrecognised\"}]\n" {
		t.Errorf("empty id json %q", r.out)
	}
	// text is encoded as JSON requires
	v = inspectJSON(t, "a\"b\\c\n\u00e9")
	if v[0]["input"] != "a\"b\\c\n\u00e9" {
		t.Errorf("input round trip: %q", v[0]["input"])
	}
}

func TestConvert(t *testing.T) {
	cases := []struct{ in, out string }{
		{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "01563e3a-b5d3-d676-4c61-efb99302bd5b"},
		{"01563e3a-b5d3-d676-4c61-efb99302bd5b", "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{"01563E3AB5D3D6764C61EFB99302BD5B", "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{"{01563E3A-B5D3-D676-4C61-EFB99302BD5B}", "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{"URN:UUID:01563e3a-b5d3-d676-4c61-efb99302bd5b", "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{"01arz3ndektsv4rrffq69g5fav", "01563e3a-b5d3-d676-4c61-efb99302bd5b"},
		{"00000000-0000-0000-0000-000000000000", "00000000000000000000000000"},
		{"ffffffff-ffff-ffff-ffff-ffffffffffff", "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"},
		{"00000000000000000000000000", "00000000-0000-0000-0000-000000000000"},
		{"7ZZZZZZZZZZZZZZZZZZZZZZZZZ", "ffffffff-ffff-ffff-ffff-ffffffffffff"},
		{"OOOOOOOOOOOOOOOOOOOOOOOOOl", "00000000-0000-0000-0000-000000000001"},
		{"00000000-0000-0000-0000-000000000001", "00000000000000000000000001"},
		{"00000000-0000-0000-c000-000000000000", convertWant(t, "00000000-0000-0000-c000-000000000000")},
	}
	for _, c := range cases {
		r := idscope(t, "convert", c.in)
		if r.out != c.out+"\n" || r.code != 0 || r.err != "" {
			t.Errorf("convert %q: %d %q %q, want %q", c.in, r.code, r.out, r.err, c.out)
		}
	}
	r := idscope(t, "convert", "--json", "01arz3ndektsv4rrffq69g5fav")
	var o map[string]string
	if err := json.Unmarshal([]byte(r.out), &o); err != nil || !strings.HasSuffix(r.out, "}\n") {
		t.Fatalf("json %q: %v", r.out, err)
	}
	want := map[string]string{"input": "01arz3ndektsv4rrffq69g5fav", "from": "ulid", "to": "uuid", "result": "01563e3a-b5d3-d676-4c61-efb99302bd5b"}
	if fmt.Sprint(o) != fmt.Sprint(want) {
		t.Errorf("got %v", o)
	}
	r = idscope(t, "--json", "convert", "{01563E3A-B5D3-D676-4C61-EFB99302BD5B}")
	want = map[string]string{"input": "{01563E3A-B5D3-D676-4C61-EFB99302BD5B}", "from": "uuid", "to": "ulid", "result": "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	o = nil
	if err := json.Unmarshal([]byte(r.out), &o); err != nil || fmt.Sprint(o) != fmt.Sprint(want) {
		t.Errorf("got %q %v", r.out, err)
	}
}

// convertWant computes the ULID of a UUID with math/big, independently of main.go.
func convertWant(t *testing.T, uuid string) string {
	t.Helper()
	n, ok := new(big.Int).SetString(strings.ReplaceAll(uuid, "-", ""), 16)
	if !ok {
		t.Fatal("bad uuid")
	}
	return bigULID(n)
}

func bigULID(n *big.Int) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	out := make([]byte, 26)
	n = new(big.Int).Set(n)
	m := new(big.Int)
	for i := 25; i >= 0; i-- {
		n.DivMod(n, big.NewInt(32), m)
		out[i] = alphabet[m.Int64()]
	}
	return string(out)
}

func fillRand(rng *rand.Rand, u *id128) {
	for i := range u {
		u[i] = byte(rng.Intn(256))
	}
}

func TestConvertAgainstBig(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		var u id128
		fillRand(rng, &u)
		if i%7 == 0 {
			u[0] = 0
		}
		text := u.uuidString()
		want := bigULID(new(big.Int).SetBytes(u[:]))
		if got := u.ulidString(); got != want {
			t.Fatalf("%s: ulid %s, want %s", text, got, want)
		}
		back, e := parseULID(want)
		if e != "" || back != u {
			t.Fatalf("%s: round trip %v %q", want, back, e)
		}
		if p := parseID(strings.ToUpper(text)); p.kind != "uuid" || p.u != u {
			t.Fatalf("parse %s", text)
		}
	}
	// there and back through the program, with several forms
	for i := 0; i < 6; i++ {
		var u id128
		fillRand(rng, &u)
		text := u.uuidString()
		ulid := idscope(t, "convert", "{"+text+"}").out
		back := idscope(t, "convert", strings.TrimSpace(ulid)).out
		if strings.TrimSpace(ulid) != u.ulidString() || strings.TrimSpace(back) != text {
			t.Errorf("%s -> %q -> %q", text, ulid, back)
		}
	}
}

func TestConvertInvalid(t *testing.T) {
	for _, in := range []string{"", "nope", "80000000000000000000000000", "01ARZ3NDEKTSV4RRFFQ69G5FAU", "{" + "017f22e279b07cc398c4dc0c0c07398f" + "}", "01563e3a-b5d3-d676-4c61-efb99302bd5"} {
		for _, args := range [][]string{{"convert", in}, {"convert", "--json", in}} {
			r := idscope(t, args...)
			if r.code != 1 || r.out != "" || !strings.HasPrefix(r.err, "idscope: ") || !strings.Contains(r.err, in) {
				t.Errorf("%q: code %d stdout %q stderr %q", args, r.code, r.out, r.err)
			}
		}
	}
}

var (
	uuid7Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	uuid4Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	ulidRe  = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
)

func lines(t *testing.T, out string) []string {
	t.Helper()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output does not end with a newline: %q", out)
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

func near(t *testing.T, ms uint64, before, after time.Time) {
	t.Helper()
	lo, hi := before.UnixMilli()-5000, after.UnixMilli()+5000
	if int64(ms) < lo || int64(ms) > hi {
		t.Errorf("timestamp %d ms is not near the clock (%d..%d)", ms, lo, hi)
	}
}

func TestNewDefault(t *testing.T) {
	before := time.Now()
	r := idscope(t, "new")
	after := time.Now()
	ids := lines(t, r.out)
	if r.code != 0 || len(ids) != 1 || !uuid7Re.MatchString(ids[0]) || r.err != "" {
		t.Fatalf("got %d %q %q", r.code, r.out, r.err)
	}
	u, _ := parseUUID(ids[0])
	near(t, u.unixMillis(), before, after)
}

func checkRun(t *testing.T, kind string, n int) []string {
	t.Helper()
	before := time.Now()
	r := idscope(t, "new", "--kind", kind, "--count", fmt.Sprint(n))
	after := time.Now()
	if r.code != 0 || r.err != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.err)
	}
	ids := lines(t, r.out)
	if len(ids) != n {
		t.Fatalf("%d ids, want %d", len(ids), n)
	}
	for i, id := range ids {
		switch kind {
		case "uuid4":
			if !uuid4Re.MatchString(id) {
				t.Fatalf("bad uuid4 %q", id)
			}
		case "uuid7":
			if !uuid7Re.MatchString(id) {
				t.Fatalf("bad uuid7 %q", id)
			}
			u, _ := parseUUID(id)
			near(t, u.unixMillis(), before, after)
		case "ulid":
			if !ulidRe.MatchString(id) {
				t.Fatalf("bad ulid %q", id)
			}
			u, e := parseULID(id)
			if e != "" {
				t.Fatalf("ulid %q: %s", id, e)
			}
			near(t, u.unixMillis(), before, after)
		}
		if kind != "uuid4" && i > 0 && !(ids[i-1] < id) {
			t.Fatalf("ids %d and %d are not strictly increasing: %s %s", i-1, i, ids[i-1], id)
		}
	}
	return ids
}

func TestNewOrder(t *testing.T) {
	for _, kind := range []string{"uuid7", "ulid"} {
		for _, n := range []int{1, 2, 50, 1000, 1000} {
			checkRun(t, kind, n)
		}
	}
}

func TestNewUUID4(t *testing.T) {
	for _, n := range []int{1, 1000} {
		ids := checkRun(t, "uuid4", n)
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("duplicate %s", id)
			}
			seen[id] = true
		}
	}
}

func TestNewInspects(t *testing.T) {
	for _, kind := range []string{"uuid4", "uuid7", "ulid"} {
		r := idscope(t, "new", "--kind", kind, "--count", "3")
		args := append([]string{"inspect"}, lines(t, r.out)...)
		ri := idscope(t, args...)
		if ri.code != 0 || strings.Count(ri.out, "valid: true") != 3 {
			t.Errorf("%s: %d %s", kind, ri.code, ri.out)
		}
	}
}

func TestNewJSON(t *testing.T) {
	for _, kind := range []string{"uuid4", "uuid7", "ulid"} {
		r := idscope(t, "new", "--json", "--count", "20", "--kind", kind)
		if strings.Count(r.out, "\n") != 1 || !strings.HasSuffix(r.out, "}\n") {
			t.Fatalf("not one line: %q", r.out)
		}
		var o map[string]any
		if err := json.Unmarshal([]byte(r.out), &o); err != nil {
			t.Fatal(err)
		}
		if len(o) != 2 || o["kind"] != kind {
			t.Fatalf("got %v", o)
		}
		ids, _ := o["ids"].([]any)
		if len(ids) != 20 {
			t.Fatalf("%d ids", len(ids))
		}
		for i, x := range ids {
			s, ok := x.(string)
			if !ok || (kind != "uuid4" && i > 0 && !(ids[i-1].(string) < s)) {
				t.Fatalf("bad id %v at %d", x, i)
			}
		}
	}
	r := idscope(t, "new", "--json")
	var o struct {
		Kind string
		IDs  []string
	}
	if err := json.Unmarshal([]byte(r.out), &o); err != nil || o.Kind != "uuid7" || len(o.IDs) != 1 {
		t.Errorf("default json %q %v", r.out, err)
	}
}

func TestNewOptionValues(t *testing.T) {
	for _, c := range []string{"0", "1001", "-1", "+1", "01", "00", "1.0", "1e2", " 1", "1 ", "0x10", "abc", "", "٣", "99999999999999999999", "10000"} {
		r := idscope(t, "new", "--count", c)
		if r.code != 2 || r.out != "" || !strings.HasPrefix(r.err, "idscope: ") || !strings.Contains(r.err, c) {
			t.Errorf("--count %q: %d %q %q", c, r.code, r.out, r.err)
		}
	}
	for _, c := range []string{"1", "2", "9", "10", "999", "1000"} {
		r := idscope(t, "new", "--count", c)
		if r.code != 0 || fmt.Sprint(len(lines(t, r.out))) != c {
			t.Errorf("--count %s: %d", c, r.code)
		}
	}
	for _, k := range []string{"UUID4", "Ulid", "ULID", "uuid", "uuid1", "uuid6", "uuid8", "uuid7 ", " uuid7", "", "guid", "--json", "ulid\n"} {
		r := idscope(t, "new", "--kind", k)
		if r.code != 2 || r.out != "" || !strings.HasPrefix(r.err, "idscope: ") || !strings.Contains(r.err, k) {
			t.Errorf("--kind %q: %d %q %q", k, r.code, r.out, r.err)
		}
	}
	for _, k := range []string{"uuid4", "uuid7", "ulid"} {
		if r := idscope(t, "new", "--kind", k); r.code != 0 {
			t.Errorf("--kind %s: %d", k, r.code)
		}
	}
}

func TestUsageErrors(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	cases := []struct {
		args []string
		text string // must appear in stderr
	}{
		{nil, ""},
		{[]string{"--json"}, ""},
		{[]string{"--kind", "uuid4"}, ""},
		{[]string{"bogus"}, "bogus"},
		{[]string{"bogus", id}, "bogus"},
		{[]string{"Inspect", id}, "Inspect"},
		{[]string{""}, ""},
		{[]string{"--json", "frobnicate"}, "frobnicate"},
		{[]string{"inspect"}, ""},
		{[]string{"inspect", "--json"}, ""},
		{[]string{"convert"}, ""},
		{[]string{"convert", id, id}, ""},
		{[]string{"convert", "a", "b", "c"}, ""},
		{[]string{"new", id}, ""},
		{[]string{"new", ""}, ""},
		{[]string{"new", "--kind", "ulid", id}, ""},
		{[]string{"inspect", "--kind", "uuid4", id}, "--kind"},
		{[]string{"inspect", id, "--count", "2"}, "--count"},
		{[]string{"convert", "--kind", "ulid", id}, "--kind"},
		{[]string{"convert", id, "--count", "2"}, "--count"},
		{[]string{"new", "--kind"}, "--kind"},
		{[]string{"new", "--count"}, "--count"},
		{[]string{"--kind", "new"}, ""},
		{[]string{"new", "--count", "1", "--count", "1"}, "--count"},
		{[]string{"new", "--kind", "ulid", "--kind", "ulid"}, "--kind"},
		{[]string{"new", "--count", "3", "--json", "--count", "4"}, "--count"},
		{[]string{"new", "--count=3"}, "--count=3"},
		{[]string{"new", "--kind=ulid"}, "--kind=ulid"},
		{[]string{"inspect", "--", id}, "--"},
		{[]string{"inspect", "-", id}, "-"},
		{[]string{"inspect", "-x", id}, "-x"},
		{[]string{"inspect", "-" + id}, "-" + id},
		{[]string{"inspect", "-00000000-0000-0000-0000-000000000000"}, "-00000000"},
		{[]string{"convert", "-" + id}, "-" + id},
		{[]string{"new", "-n"}, "-n"},
		{[]string{"new", "-1"}, "-1"},
		{[]string{"--help", id}, "--help"},
		{[]string{"--help", "--json"}, "--help"},
		{[]string{"--json", "--help"}, "--help"},
		{[]string{"new", "--help"}, "--help"},
		{[]string{"inspect", "-h"}, "-h"},
		{[]string{"-h", "-h"}, "-h"},
		{[]string{"--help", "--help"}, "--help"},
		{[]string{"-h", "new"}, "-h"},
		{[]string{"--version", "new"}, "--version"},
		{[]string{"new", "--version"}, "--version"},
		{[]string{"--version", "--json"}, "--version"},
		{[]string{"--version", "--version"}, "--version"},
		{[]string{"--JSON", "new"}, "--JSON"},
		{[]string{"-json", "new"}, "-json"},
		{[]string{"new", "--count", "0", "--json"}, "0"},
		{[]string{"new", "--kind", "--count", "2"}, "--count"},
	}
	for _, c := range cases {
		r := idscope(t, c.args...)
		if r.code != 2 || r.out != "" || !strings.HasPrefix(r.err, "idscope: ") || !strings.Contains(r.err, c.text) {
			t.Errorf("%q: code %d stdout %q stderr %q", c.args, r.code, r.out, r.err)
		}
	}
}

func TestHelpAndVersion(t *testing.T) {
	for _, a := range []string{"--help", "-h"} {
		r := idscope(t, a)
		first, _, _ := strings.Cut(r.out, "\n")
		if r.code != 0 || r.err != "" || !strings.HasPrefix(first, "usage: idscope") {
			t.Errorf("%s: %d %q %q", a, r.code, r.out, r.err)
		}
		for _, w := range []string{"inspect", "new", "convert"} {
			if !strings.Contains(r.out, w) {
				t.Errorf("%s: usage lacks %s", a, w)
			}
		}
	}
	r := idscope(t, "--version")
	if r.code != 0 || r.out != "idscope 0.1.0\n" || r.err != "" {
		t.Errorf("--version: %d %q %q", r.code, r.out, r.err)
	}
}

func TestOptionsAnywhere(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	same := [][]string{
		{"inspect", id, "--json"},
		{"--json", "inspect", id},
		{"inspect", "--json", id},
		{"--json", "inspect", "--json", id, "--json"},
	}
	want := idscope(t, same[0]...).out
	if !strings.HasPrefix(want, "[{") {
		t.Fatalf("not json: %q", want)
	}
	for _, a := range same {
		if r := idscope(t, a...); r.out != want || r.code != 0 {
			t.Errorf("%q: %d %q", a, r.code, r.out)
		}
	}
	for _, a := range [][]string{
		{"new", "--count", "3", "--kind", "ulid"},
		{"--kind", "ulid", "--count", "3", "new"},
		{"--count", "3", "new", "--kind", "ulid"},
		{"new", "--count", "3", "--json", "--kind", "ulid", "--json"},
		{"--json", "--kind", "ulid", "new", "--count", "3"},
	} {
		r := idscope(t, a...)
		if r.code != 0 || r.err != "" {
			t.Errorf("%q: %d %q", a, r.code, r.err)
		}
		if strings.Contains(strings.Join(a, " "), "--json") {
			if !strings.HasPrefix(r.out, `{"kind":"ulid","ids":[`) {
				t.Errorf("%q: %q", a, r.out)
			}
		} else if n := len(lines(t, r.out)); n != 3 {
			t.Errorf("%q: %d lines", a, n)
		}
	}
	// a value is taken whatever it holds, even a command name
	r := idscope(t, "convert", "--json", id)
	if r.code != 0 {
		t.Errorf("convert --json: %d", r.code)
	}
}

func TestNoStdinNoFiles(t *testing.T) {
	dir := t.TempDir()
	// standard input is a pipe nobody writes to and nobody closes: a read would hang
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	defer pw.Close()
	for _, args := range [][]string{{"new", "--count", "5"}, {"new", "--kind", "ulid", "--json"}, {"convert", "01ARZ3NDEKTSV4RRFFQ69G5FAV"}, {"inspect", "nope"}, {"bogus"}, {"--help"}, {"--version"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		cmd := exec.CommandContext(ctx, binPath, args...)
		cmd.Dir = dir
		cmd.Stdin = pr
		cmd.Run()
		if ctx.Err() != nil {
			t.Errorf("%q did not finish: it reads standard input?", args)
		}
		cancel()
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Errorf("files written: %v %v", entries, err)
	}
}

func fixedMaker(bits uint, clock *[]uint64, fresh [][2]uint64) *maker {
	i := 0
	return &maker{
		bits: bits,
		now: func() uint64 {
			v := (*clock)[0]
			if len(*clock) > 1 {
				*clock = (*clock)[1:]
			}
			return v
		},
		fresh: func(uint) (uint64, uint64) {
			v := fresh[i]
			i++
			return v[0], v[1]
		},
	}
}

func TestMakerRules(t *testing.T) {
	clock := []uint64{100, 100, 100, 99, 101, 101}
	m := fixedMaker(80, &clock, [][2]uint64{{7, 5}, {1, 1}, {2, 2}})
	type step struct{ t, hi, lo uint64 }
	want := []step{
		{100, 7, 5}, // first id: now and fresh R
		{100, 7, 6}, // same millisecond: R + 1
		{100, 7, 7}, // same again
		{100, 7, 8}, // the clock went back: keep T, R + 1
		{101, 1, 1}, // the clock moved on: now and fresh R
		{101, 1, 2},
	}
	for i, w := range want {
		t1, hi, lo := m.next()
		if (step{t1, hi, lo}) != w {
			t.Errorf("step %d: got %v, want %v", i, step{t1, hi, lo}, w)
		}
	}
	// carry from the low word into the high one
	clock = []uint64{5}
	m = fixedMaker(80, &clock, [][2]uint64{{3, ^uint64(0)}})
	m.next()
	if _, hi, lo := m.next(); hi != 4 || lo != 0 {
		t.Errorf("carry: %d %d", hi, lo)
	}
	// R + 1 does not fit: T + 1 and a fresh R, for both widths
	for _, bits := range []uint{74, 80} {
		maxHi := uint64(1)<<(bits-64) - 1
		clock = []uint64{9}
		m = fixedMaker(bits, &clock, [][2]uint64{{maxHi, ^uint64(0)}, {0, 42}})
		m.next()
		tt, hi, lo := m.next()
		if tt != 10 || hi != 0 || lo != 42 {
			t.Errorf("bits %d overflow: %d %d %d", bits, tt, hi, lo)
		}
		tt, hi, lo = m.next()
		if tt != 10 || hi != 0 || lo != 43 {
			t.Errorf("bits %d after overflow: %d %d %d", bits, tt, hi, lo)
		}
	}
}

func TestMakerIDsIncrease(t *testing.T) {
	for _, bits := range []uint{74, 80} {
		for _, edge := range []bool{false, true} {
			clock := []uint64{1000}
			start := [2]uint64{0, 0}
			if edge {
				// crossing the 62-bit and 64-bit boundaries of R
				start = [2]uint64{0, 1<<62 - 3}
			}
			m := fixedMaker(bits, &clock, [][2]uint64{start, {1, 1 << 40}})
			prev := ""
			for i := 0; i < 12; i++ {
				tt, hi, lo := m.next()
				var s string
				if bits == 74 {
					s = buildUUID7(tt, hi, lo).uuidString()
					if !uuid7Re.MatchString(s) {
						t.Fatalf("bad uuid7 %s", s)
					}
				} else {
					s = buildULID(tt, hi, lo).ulidString()
				}
				if s <= prev {
					t.Fatalf("bits %d edge %v: %s after %s", bits, edge, s, prev)
				}
				prev = s
			}
		}
	}
	// the fields of a built uuid7 and ulid
	u := buildUUID7(0x017f22e279b0, 0x3ff, ^uint64(0))
	if got := u.uuidString(); got != "017f22e2-79b0-7fff-bfff-ffffffffffff" {
		t.Errorf("uuid7 all ones: %s", got)
	}
	u = buildUUID7(0x017f22e279b0, 0, 0)
	if got := u.uuidString(); got != "017f22e2-79b0-7000-8000-000000000000" {
		t.Errorf("uuid7 zeros: %s", got)
	}
	u = buildUUID7(1, 0, 1<<62)
	if got := u.uuidString(); got != "00000000-0001-7001-8000-000000000000" {
		t.Errorf("uuid7 rand_a 1: %s", got)
	}
	if got := buildULID(0xffffffffffff, 0xffff, ^uint64(0)).ulidString(); got != "7ZZZZZZZZZZZZZZZZZZZZZZZZZ" {
		t.Errorf("ulid all ones: %s", got)
	}
	if got := buildULID(1, 0, 0).ulidString(); got != "00000000010000000000000000" {
		t.Errorf("ulid ms 1: %s", got)
	}
}

func TestValidCount(t *testing.T) {
	for s, want := range map[string]bool{"1": true, "1000": true, "999": true, "0": false, "1001": false, "01": false, "": false, "-1": false, "+5": false, "5a": false} {
		if _, ok := validCount(s); ok != want {
			t.Errorf("validCount(%q) = %v", s, ok)
		}
	}
}

func TestRealMainWritesNothingOnUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := realMain([]string{"inspect", "x", "--kind", "ulid"}, &out, &errb); code != 2 || out.Len() != 0 || !strings.HasPrefix(errb.String(), "idscope: ") {
		t.Errorf("%d %q %q", code, out.String(), errb.String())
	}
}
