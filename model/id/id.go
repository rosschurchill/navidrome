package id

import (
	"fmt"
	"math/big"
	"strings"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/navidrome/navidrome/log"
	"golang.org/x/crypto/sha3"
)

func NewRandom() string {
	id, err := gonanoid.Generate("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", 22)
	if err != nil {
		log.Error("Could not generate new ID", err)
	}
	return id
}

// NewHash generates a deterministic ID from input data using SHA3-256 (post-quantum resistant).
// Output is truncated to 128 bits for format compatibility with existing 22-char base62 IDs.
func NewHash(data ...string) string {
	hash := sha3.New256()
	for _, d := range data {
		hash.Write([]byte(d))
		hash.Write([]byte(string('\u200b')))
	}
	// Truncate to 16 bytes (128 bits) for format compatibility
	h := hash.Sum(nil)[:16]
	bi := big.NewInt(0)
	bi.SetBytes(h)
	s := bi.Text(62)
	return fmt.Sprintf("%022s", s)
}

func NewTagID(name, value string) string {
	return NewHash(strings.ToLower(name), strings.ToLower(value))
}
