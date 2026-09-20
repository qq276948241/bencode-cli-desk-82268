package bencode

import (
	"fmt"
	"strconv"
	"strings"
)

// PathError reports that a path segment could not be resolved.
type PathError struct {
	Path    string
	Segment string
	Reason  string
}

func (e *PathError) Error() string {
	return fmt.Sprintf("path %q: %s (%q)", e.Path, e.Reason, e.Segment)
}

// Get resolves a dotted path with optional [index] accessors, e.g.
//
//	info.name
//	info.files[0].path
//	a.b[12].c
//
// A literal dot or bracket inside a dict key is not supported; such keys can
// be reached via the JSON representation instead.
func Get(root *Value, path string) (*Value, error) {
	if path == "" {
		return root, nil
	}
	segments, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	cur := root
	for _, seg := range segments {
		switch {
		case seg.key != "":
			dict, ok := cur.V.(*Dict)
			if !ok {
				return nil, &PathError{Path: path, Segment: seg.key, Reason: "value is not a dict"}
			}
			next, ok := dict.Values[seg.key]
			if !ok {
				return nil, &PathError{Path: path, Segment: seg.key, Reason: "key not found"}
			}
			cur = next
		case seg.index >= 0:
			list, ok := cur.V.([]*Value)
			if !ok {
				return nil, &PathError{Path: path, Segment: "[" + strconv.Itoa(seg.index) + "]", Reason: "value is not a list"}
			}
			if seg.index >= len(list) {
				return nil, &PathError{Path: path, Segment: "[" + strconv.Itoa(seg.index) + "]", Reason: fmt.Sprintf("index out of range (length %d)", len(list))}
			}
			cur = list[seg.index]
		}
	}
	return cur, nil
}

type segment struct {
	key   string
	index int
}

func parsePath(path string) ([]segment, error) {
	var segs []segment
	var key strings.Builder
	flush := func() {
		if key.Len() > 0 {
			segs = append(segs, segment{key: key.String(), index: -1})
			key.Reset()
		}
	}
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '.':
			flush()
		case '[':
			flush()
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("path %q: missing closing ]", path)
			}
			end += i
			numStr := path[i+1 : end]
			n, err := strconv.Atoi(numStr)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("path %q: invalid list index %q", path, numStr)
			}
			segs = append(segs, segment{index: n})
			i = end
			if i+1 < len(path) && path[i+1] != '.' && path[i+1] != '[' {
				return nil, fmt.Errorf("path %q: expected '.' or '[' after ]", path)
			}
		default:
			key.WriteByte(path[i])
		}
	}
	flush()
	if len(segs) == 0 {
		return nil, fmt.Errorf("path %q is empty", path)
	}
	return segs, nil
}
