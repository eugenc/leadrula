// Package handlerid generates short unique handler IDs (B-, P-, PB-, PL-, C- prefixes).
package handlerid

import (
	"crypto/rand"
	"math/big"
)

const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

const (
	DefaultLength  = 5
	ContractLength = 6
)

// Generate returns a handler ID with the given prefix and DefaultLength suffix (accounts).
func Generate(prefix string) string {
	return GenerateN(prefix, DefaultLength)
}

// GenerateContract returns a contract handler ID (C- + ContractLength suffix, ~1.5B namespace).
func GenerateContract() string {
	return GenerateN("C", ContractLength)
}

// GenerateN returns prefix + "-" + n random alphabet characters.
func GenerateN(prefix string, n int) string {
	b := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := range b {
		rn, _ := rand.Int(rand.Reader, max)
		b[i] = alphabet[rn.Int64()]
	}
	return prefix + "-" + string(b)
}
