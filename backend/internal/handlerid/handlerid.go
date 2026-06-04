// Package handlerid generates short unique handler IDs (B-, P-, PB-, PL-, C- prefixes).
package handlerid

import (
	"crypto/rand"
	"math/big"
)

const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// Generate returns a handler ID with the given single-letter prefix (B, P, or C).
func Generate(prefix string) string {
	b := make([]byte, 5)
	max := big.NewInt(int64(len(alphabet)))
	for i := range b {
		n, _ := rand.Int(rand.Reader, max)
		b[i] = alphabet[n.Int64()]
	}
	return prefix + "-" + string(b)
}
