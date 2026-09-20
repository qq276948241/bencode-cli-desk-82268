package bencode

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// StringStyle controls how byte strings are rendered.
type StringStyle int

const (
	StyleAuto  StringStyle = iota // printable UTF-8 as text, otherwise hex
	StyleHex                      // always hex
	StyleASCII                    // printable ASCII quoted, otherwise hex
)

// Render pretty-prints a decoded value as an indented, human-readable tree.
func Render(v *Value, style StringStyle) string {
	var sb strings.Builder
	renderValue(&sb, v, "", style)
	return sb.String()
}

func renderValue(sb *strings.Builder, v *Value, indent string, style StringStyle) {
	switch t := v.V.(type) {
	case int64:
		fmt.Fprintf(sb, "%d\n", t)
	case []byte:
		sb.WriteString(renderBytes(t, style))
		sb.WriteByte('\n')
	case []*Value:
		if len(t) == 0 {
			sb.WriteString("[]\n")
			return
		}
		sb.WriteString("[\n")
		child := indent + "  "
		for i, item := range t {
			fmt.Fprintf(sb, "%s[%d] ", child, i)
			renderValue(sb, item, child, style)
		}
		fmt.Fprintf(sb, "%s]\n", indent)
	case *Dict:
		if len(t.Keys) == 0 {
			sb.WriteString("{}\n")
			return
		}
		sb.WriteString("{\n")
		child := indent + "  "
		for _, key := range t.Keys {
			fmt.Fprintf(sb, "%s%q: ", child, string(key))
			renderValue(sb, t.Values[string(key)], child, style)
		}
		fmt.Fprintf(sb, "%s}\n", indent)
	}
}

func renderBytes(b []byte, style StringStyle) string {
	switch style {
	case StyleHex:
		return "0x" + bytesHex(b)
	case StyleASCII:
		if isPrintableASCII(b) {
			return quoteASCII(b)
		}
		return "0x" + bytesHex(b)
	default:
		if utf8.Valid(b) && isMostlyPrintable(b) {
			return quoteText(b)
		}
		return "0x" + bytesHex(b)
	}
}

func isPrintableASCII(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

// isMostlyPrintable treats a UTF-8 string as text if it has no control
// characters other than common whitespace.
func isMostlyPrintable(b []byte) bool {
	for _, r := range string(b) {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 0x20 {
			return false
		}
	}
	return true
}

func quoteASCII(b []byte) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, c := range b {
		switch c {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		default:
			sb.WriteByte(c)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

func quoteText(b []byte) string {
	return fmt.Sprintf("%q", string(b))
}
