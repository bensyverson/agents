package module

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashLength is the number of hex characters kept from the SHA-256 digest.
// Six is enough to spot a stale region by eye and short enough to read in a marker.
const HashLength = 6

// Hash is the content address of a module body: the first HashLength hex
// characters of its SHA-256 digest. It is what appears after "@" in a region
// marker, so staleness is a string compare.
func Hash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:HashLength]
}
