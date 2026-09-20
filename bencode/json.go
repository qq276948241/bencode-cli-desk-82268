package bencode

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf8"
)

// ByteTag marks a JSON value that must round-trip as raw bytes rather than a
// UTF-8 string. A bencode byte string that is valid UTF-8 is emitted as a
// plain JSON string; non-UTF-8 payloads become {"@hex": "...."}.
type ByteTag struct {
	Hex string `json:"@hex"`
}

// ToNative converts a decoded tree into JSON-friendly Go types
// (int64 / string | {"@hex":...} / []interface{} / map[string]interface{}).
func ToNative(v *Value) interface{} {
	switch t := v.V.(type) {
	case int64:
		return t
	case []byte:
		if utf8.Valid(t) {
			return string(t)
		}
		return ByteTag{Hex: hex.EncodeToString(t)}
	case []*Value:
		out := make([]interface{}, 0, len(t))
		for _, item := range t {
			out = append(out, ToNative(item))
		}
		return out
	case *Dict:
		out := make(map[string]interface{}, len(t.Keys))
		for _, key := range t.Keys {
			out[string(key)] = ToNative(t.Values[string(key)])
		}
		return out
	default:
		return nil
	}
}

// FromJSON parses the intermediate JSON representation back into a *Value.
func FromJSON(data []byte) (*Value, error) {
	var raw interface{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	var extra interface{}
	if err := dec.Decode(&extra); err == nil {
		return nil, fmt.Errorf("invalid JSON: trailing data")
	}
	return fromNative(raw)
}

func fromNative(raw interface{}) (*Value, error) {
	switch t := raw.(type) {
	case nil:
		return nil, fmt.Errorf("null is not representable in bencode")
	case bool:
		return nil, fmt.Errorf("boolean %v is not representable in bencode; use 0/1 integer", t)
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return nil, fmt.Errorf("number %q must be an integer", t.String())
		}
		return &Value{V: n, Kind: KindInt}, nil
	case string:
		return &Value{V: []byte(t), Kind: KindBytes}, nil
	case map[string]interface{}:
		if len(t) == 1 {
			if h, ok := t["@hex"]; ok {
				hs, _ := h.(string)
				b, err := hex.DecodeString(hs)
				if err != nil {
					return nil, fmt.Errorf("invalid @hex payload: %w", err)
				}
				return &Value{V: b, Kind: KindBytes}, nil
			}
		}
		if _, ok := t["@hex"]; ok && len(t) != 1 {
			return nil, fmt.Errorf("@hex tag must be the only key in its object")
		}
		dict := &Dict{Values: make(map[string]*Value, len(t))}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child, err := fromNative(t[k])
			if err != nil {
				return nil, err
			}
			dict.Keys = append(dict.Keys, []byte(k))
			dict.Values[k] = child
		}
		return &Value{V: dict, Kind: KindDict}, nil
	case []interface{}:
		items := make([]*Value, 0, len(t))
		for _, item := range t {
			child, err := fromNative(item)
			if err != nil {
				return nil, err
			}
			items = append(items, child)
		}
		return &Value{V: items, Kind: KindList}, nil
	default:
		return nil, fmt.Errorf("unsupported JSON type %T", raw)
	}
}
