package bencode

import (
	"fmt"
	"sort"
	"strconv"
)

// Encode serializes a decoded *Value back to bencode. Dict keys are emitted
// in lexicographic byte order as the spec requires.
func Encode(v *Value) ([]byte, error) {
	var out []byte
	if err := encodeInto(&out, v); err != nil {
		return nil, err
	}
	return out, nil
}

func encodeInto(out *[]byte, v *Value) error {
	switch t := v.V.(type) {
	case int64:
		*out = append(*out, 'i')
		*out = strconv.AppendInt(*out, t, 10)
		*out = append(*out, 'e')
	case []byte:
		*out = strconv.AppendInt(*out, int64(len(t)), 10)
		*out = append(*out, ':')
		*out = append(*out, t...)
	case []*Value:
		*out = append(*out, 'l')
		for _, item := range t {
			if err := encodeInto(out, item); err != nil {
				return err
			}
		}
		*out = append(*out, 'e')
	case *Dict:
		*out = append(*out, 'd')
		keys := make([][]byte, len(t.Keys))
		copy(keys, t.Keys)
		sort.Slice(keys, func(i, j int) bool { return compareBytes(keys[i], keys[j]) < 0 })
		for _, key := range keys {
			*out = strconv.AppendInt(*out, int64(len(key)), 10)
			*out = append(*out, ':')
			*out = append(*out, key...)
			if err := encodeInto(out, t.Values[string(key)]); err != nil {
				return err
			}
		}
		*out = append(*out, 'e')
	default:
		return fmt.Errorf("bencode: cannot encode value of type %T", v.V)
	}
	return nil
}
