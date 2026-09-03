// Package review: pure-function helpers for ID generation and the
// terminal-window clock. Kept tiny on purpose — the service is the
// orchestration layer.
package review

import (
	"crypto/rand"
	"encoding/hex"
)

// IDGenerator produces UUIDs for the review rows. The fakes in tests
// use a counter; production wires defaultIDGen from this package.
type IDGenerator interface {
	String() string
}

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
