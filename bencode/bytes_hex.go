package bencode

import "encoding/hex"

func bytesHex(b []byte) string {
	return hex.EncodeToString(b)
}
