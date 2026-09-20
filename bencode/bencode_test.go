package bencode

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestDecodeBasic(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"i42e", "42"},
		{"i0e", "0"},
		{"i-5e", "-5"},
		{"0:", ""},
		{"4:spam", "spam"},
		{"le", "[]"},
		{"li1ei2ee", "[1 2]"},
		{"de", "{}"},
		{"d3:cow3:moo4:spami7ee", "{cow=moo spam=7}"},
	}
	for _, c := range cases {
		v, err := Decode([]byte(c.in))
		if err != nil {
			t.Fatalf("Decode(%q): %v", c.in, err)
		}
		got := compact(v)
		if got != c.want {
			t.Fatalf("Decode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func compact(v *Value) string {
	switch t := v.V.(type) {
	case int64:
		return strconv.FormatInt(t, 10)
	case []byte:
		return string(t)
	case []*Value:
		parts := make([]string, len(t))
		for i, c := range t {
			parts[i] = compact(c)
		}
		return "[" + strings.Join(parts, " ") + "]"
	case *Dict:
		var b strings.Builder
		b.WriteString("{")
		for i, k := range t.Keys {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(string(k) + "=" + compact(t.Values[string(k)]))
		}
		b.WriteString("}")
		return b.String()
	}
	return ""
}

func TestInvalidInputs(t *testing.T) {
	bad := []string{
		"",
		"x",
		"i12",
		"i01e",
		"i-0e",
		"i--1e",
		"i1",
		"4:spa",
		"04:spam",
		"99999999999999999999:x",
		"l",
		"d3:cow",
		"d3:cowi1e3:cowi2ee",  // duplicate
		"d4:spami1e3:cowi2ee", // unsorted
		"i1eGARBAGE",
		"l" + strings.Repeat("l", MaxDepth) + "ee",
	}
	for _, in := range bad {
		if _, err := Decode([]byte(in)); err == nil {
			t.Fatalf("expected error decoding %q", in)
		}
	}
}

func TestPathGet(t *testing.T) {
	// d5:filesld4:pathl3:abceee4:info d4:name4:ubuntuee
	root, err := Decode([]byte("d5:filesld4:pathl3:abceee4:infod4:name6:ubuntuee"))
	if err != nil {
		t.Fatal(err)
	}
	ok := func(path, want string) {
		t.Helper()
		v, err := Get(root, path)
		if err != nil {
			t.Fatalf("Get(%q): %v", path, err)
		}
		if got := compact(v); got != want {
			t.Fatalf("Get(%q) = %q want %q", path, got, want)
		}
	}
	ok("info.name", "ubuntu")
	ok("files[0].path[0]", "abc")

	missing := []string{"nope", "info.x", "files[5]", "info.name[0]", "files[x]"}
	for _, p := range missing {
		if _, err := Get(root, p); err == nil {
			t.Fatalf("expected error for path %q", p)
		}
	}
}

// sampleSingleFileTorrent is a canonical (sorted-key) single-file torrent.
const sampleSingleFileTorrent = "d" +
	"8:announce35:http://tracker.example.com/announce" +
	"4:infod" +
	"6:lengthi12345e" +
	"4:name10:ubuntu.iso" +
	"12:piece lengthi32768e" +
	"6:pieces20:0123456789abcdefghij" +
	"ee"

func TestRoundTripPreservesInfohash(t *testing.T) {
	orig := []byte(sampleSingleFileTorrent)
	root, err := Decode(orig)
	if err != nil {
		t.Fatal(err)
	}
	h1, err := InfoHash(root)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := Encode(root)
	if err != nil {
		t.Fatal(err)
	}
	if string(rebuilt) != string(orig) {
		t.Fatalf("round-trip mismatch:\n got %x\nwant %x", rebuilt, orig)
	}
	root2, err := Decode(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := InfoHash(root2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("infohash changed: %s vs %s", h1, h2)
	}
	if len(h1) != 40 {
		t.Fatalf("infohash not 40 hex chars: %q", h1)
	}
	if h1 != strings.ToLower(h1) {
		t.Fatalf("infohash not lowercase: %q", h1)
	}
	// pieces must survive the JSON intermediate representation losslessly.
	native := ToNative(root)
	js, err := json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	fromJS, err := FromJSON(js)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt2, err := Encode(fromJS)
	if err != nil {
		t.Fatal(err)
	}
	if string(rebuilt2) != string(orig) {
		t.Fatalf("JSON round-trip mismatch:\n got %x\nwant %x", rebuilt2, orig)
	}
}

func TestHexTagRoundTrip(t *testing.T) {
	raw := []byte("d4:data4:\xff\xfe\x00\x01e")
	root, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	js, _ := json.Marshal(ToNative(root))
	if !strings.Contains(string(js), "@hex") {
		t.Fatalf("expected @hex tag in %s", js)
	}
	if want := `"` + hex.EncodeToString([]byte{0xff, 0xfe, 0x00, 0x01}) + `"`; !strings.Contains(string(js), want) {
		t.Fatalf("hex payload missing: %s", js)
	}
	back, err := FromJSON(js)
	if err != nil {
		t.Fatal(err)
	}
	re, err := Encode(back)
	if err != nil {
		t.Fatal(err)
	}
	if string(re) != string(raw) {
		t.Fatalf("hex round-trip mismatch: %x vs %x", re, raw)
	}
}
