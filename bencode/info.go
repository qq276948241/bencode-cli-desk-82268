package bencode

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

// InfoHash returns the lowercase hex SHA-1 of the raw, encoded info dict
// (the BitTorrent v1 infohash used in magnet links).
func InfoHash(root *Value) (string, error) {
	rootDict, ok := root.V.(*Dict)
	if !ok {
		return "", fmt.Errorf("top-level value is not a dict")
	}
	info, ok := rootDict.Values["info"]
	if !ok {
		return "", fmt.Errorf("no %q key present", "info")
	}
	if info.Kind != KindDict {
		return "", fmt.Errorf("%q is not a dict", "info")
	}
	raw := info.Raw
	if len(raw) == 0 {
		raw, _ = Encode(info)
	}
	sum := sha1.Sum(raw)
	return hex.EncodeToString(sum[:]), nil
}
