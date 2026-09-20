package bencode

import (
	"errors"
	"fmt"
)

// Limits guard against hostile or malformed inputs (zip bombs are not a
// bencode concern, but absurd nesting / prefixes must be rejected).
const (
	MaxDepth    = 256
	MaxIntBytes = 64
	MaxLength   = 1 << 40 // 1 TiB declared length cap
)

// Decoder parses a bencoded byte slice while retaining the raw bytes of
// every value, which is required to compute a faithful info hash.
type Decoder struct {
	data  []byte
	pos   int
	depth int
}

// Value is a decoded bencode value.
//
// The concrete type of V is one of:
//
//	int64
//	[]byte
//	[]*Value
//	*Dict
type Value struct {
	V    interface{}
	Raw  []byte // exact bencoded bytes of this value
	Kind Kind
}

type Kind uint8

const (
	KindInt Kind = iota
	KindBytes
	KindList
	KindDict
)

// Dict preserves key order as they appear on the wire and records the raw
// encoded bytes of each entry.
type Dict struct {
	Keys   [][]byte
	Values map[string]*Value
}

func (d *Dict) Get(key string) (*Value, bool) {
	v, ok := d.Values[key]
	return v, ok
}

var (
	ErrTruncated      = errors.New("bencode: unexpected end of input")
	ErrTrailing       = errors.New("bencode: trailing bytes after top-level value")
	ErrInvalidPrefix  = errors.New("bencode: invalid length prefix")
	ErrInvalidInteger = errors.New("bencode: invalid integer encoding")
)

// Decode parses exactly one bencoded value and rejects trailing bytes.
func Decode(data []byte) (*Value, error) {
	d := &Decoder{data: data}
	v, err := d.parseValue()
	if err != nil {
		return nil, err
	}
	if d.pos != len(data) {
		return nil, fmt.Errorf("%w: %d extra byte(s)", ErrTrailing, len(data)-d.pos)
	}
	return v, nil
}

func (d *Decoder) parseValue() (*Value, error) {
	if d.pos >= len(d.data) {
		return nil, ErrTruncated
	}
	switch {
	case d.data[d.pos] == 'i':
		return d.parseInt()
	case d.data[d.pos] == 'l':
		return d.parseList()
	case d.data[d.pos] == 'd':
		return d.parseDict()
	case d.data[d.pos] >= '0' && d.data[d.pos] <= '9':
		return d.parseBytes()
	default:
		return nil, fmt.Errorf("%w: %q is not a valid type prefix", ErrInvalidPrefix, string(d.data[d.pos]))
	}
}

func (d *Decoder) parseInt() (*Value, error) {
	start := d.pos
	d.pos++ // consume 'i'
	numStart := d.pos
	end := -1
	for d.pos < len(d.data) {
		if d.data[d.pos] == 'e' {
			end = d.pos
			break
		}
		d.pos++
	}
	if end < 0 {
		return nil, ErrTruncated
	}
	body := d.data[numStart:end]
	if len(body) == 0 || len(body) > MaxIntBytes {
		return nil, fmt.Errorf("%w: empty or oversized integer", ErrInvalidInteger)
	}
	neg := body[0] == '-'
	digits := body
	if neg {
		digits = body[1:]
		if len(digits) == 0 {
			return nil, fmt.Errorf("%w: dangling minus sign", ErrInvalidInteger)
		}
	}
	if len(digits) > 1 && digits[0] == '0' {
		return nil, fmt.Errorf("%w: leading zeroes are not allowed", ErrInvalidInteger)
	}
	if !neg && len(body) > 0 && body[0] == '0' && len(body) > 1 {
		return nil, fmt.Errorf("%w: leading zeroes are not allowed", ErrInvalidInteger)
	}
	if neg && len(digits) == 1 && digits[0] == '0' {
		return nil, fmt.Errorf("%w: negative zero is not allowed", ErrInvalidInteger)
	}
	var n int64
	for _, c := range digits {
		if c < '0' || c > '9' {
			return nil, fmt.Errorf("%w: non-digit %q", ErrInvalidInteger, string(c))
		}
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	d.pos = end + 1 // consume 'e'
	return &Value{V: n, Raw: d.data[start:d.pos], Kind: KindInt}, nil
}

func (d *Decoder) parseBytes() (*Value, error) {
	start := d.pos
	colon := -1
	for d.pos < len(d.data) {
		c := d.data[d.pos]
		if c == ':' {
			colon = d.pos
			break
		}
		if c < '0' || c > '9' {
			return nil, fmt.Errorf("%w: %q in length prefix", ErrInvalidPrefix, string(c))
		}
		d.pos++
	}
	if colon < 0 {
		return nil, ErrTruncated
	}
	prefix := d.data[start:colon]
	if len(prefix) > 1 && prefix[0] == '0' {
		return nil, fmt.Errorf("%w: leading zeroes in length prefix", ErrInvalidPrefix)
	}
	var length int64
	for _, c := range prefix {
		length = length*10 + int64(c-'0')
		if length < 0 || length > MaxLength {
			return nil, fmt.Errorf("%w: declared length %d out of range", ErrInvalidPrefix, length)
		}
	}
	d.pos = colon + 1
	if int64(len(d.data)-d.pos) < length {
		return nil, fmt.Errorf("%w: declares %d bytes but only %d remain", ErrInvalidPrefix, length, len(d.data)-d.pos)
	}
	b := d.data[d.pos : d.pos+int(length)]
	d.pos += int(length)
	return &Value{V: append([]byte(nil), b...), Raw: d.data[start:d.pos], Kind: KindBytes}, nil
}

func (d *Decoder) enter() error {
	d.depth++
	if d.depth > MaxDepth {
		return fmt.Errorf("bencode: nesting exceeds depth limit %d", MaxDepth)
	}
	return nil
}

func (d *Decoder) parseList() (*Value, error) {
	start := d.pos
	if err := d.enter(); err != nil {
		return nil, err
	}
	d.pos++ // consume 'l'
	var items []*Value
	for {
		if d.pos >= len(d.data) {
			return nil, ErrTruncated
		}
		if d.data[d.pos] == 'e' {
			d.pos++
			d.depth--
			return &Value{V: items, Raw: d.data[start:d.pos], Kind: KindList}, nil
		}
		v, err := d.parseValue()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
}

func (d *Decoder) parseDict() (*Value, error) {
	start := d.pos
	if err := d.enter(); err != nil {
		return nil, err
	}
	d.pos++ // consume 'd'
	dict := &Dict{Values: make(map[string]*Value)}
	var prevKey []byte
	for {
		if d.pos >= len(d.data) {
			return nil, ErrTruncated
		}
		if d.data[d.pos] == 'e' {
			d.pos++
			d.depth--
			return &Value{V: dict, Raw: d.data[start:d.pos], Kind: KindDict}, nil
		}
		keyStart := d.pos
		keyVal, err := d.parseBytes()
		if err != nil {
			return nil, fmt.Errorf("bencode: dict key must be a byte string: %w", err)
		}
		key := keyVal.V.([]byte)
		if d.pos >= len(d.data) {
			return nil, ErrTruncated
		}
		val, err := d.parseValue()
		if err != nil {
			return nil, err
		}
		if prevKey != nil && compareBytes(key, prevKey) < 0 {
			return nil, fmt.Errorf("bencode: dict keys not in sorted order at offset %d (key %q)", keyStart, string(key))
		}
		if _, dup := dict.Values[string(key)]; dup {
			return nil, fmt.Errorf("bencode: duplicate dict key %q", string(key))
		}
		prevKey = key
		dict.Keys = append(dict.Keys, append([]byte(nil), key...))
		dict.Values[string(key)] = val
	}
}

func compareBytes(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}
