package f5

import (
	"crypto/rand"
	"encoding/hex"
)

// defaultIDGen is the production UUIDv4 generator. Imported by the F5
// service for use as a default; tests inject a counter via Config.IDGen.
type defaultIDGen struct{}

func (defaultIDGen) String() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
