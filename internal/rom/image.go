// Package rom reads and edits Mega Drive ROM images.
package rom

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

// HeaderSize is the number of bytes taken by the vector table and cartridge header.
const HeaderSize = 0x200

// MaxCartSize is the largest ROM a cartridge without a mapper can hold.
const MaxCartSize = 0x400000

// FillByte is what padding and blank cartridge space read as.
const FillByte = 0xFF

// SHA1Hex returns the lower-case hex SHA-1 of img.
func SHA1Hex(img []byte) string {
	sum := sha1.Sum(img)
	return hex.EncodeToString(sum[:])
}

// Pad returns a copy of img extended to size bytes with FillByte.
func Pad(img []byte, size int) ([]byte, error) {
	if size < len(img) {
		return nil, fmt.Errorf("image is %d bytes, larger than the requested size %d", len(img), size)
	}
	out := make([]byte, size)
	copy(out, img)
	for i := len(img); i < size; i++ {
		out[i] = FillByte
	}
	return out, nil
}

// IsPowerOfTwo reports whether n is a positive power of two.
func IsPowerOfTwo(n int) bool {
	return n > 0 && n&(n-1) == 0
}

// ValidateSize checks a requested image size against the base length and the cart limit.
func ValidateSize(baseLen, size, maxSize int) error {
	if baseLen%2 != 0 {
		return fmt.Errorf("base image is %d bytes, not a valid Mega Drive ROM (odd length)", baseLen)
	}
	if size%2 != 0 {
		return fmt.Errorf("size %d is odd; ROM images must have an even length", size)
	}
	if size < baseLen {
		return fmt.Errorf("size %d is smaller than the base image (%d bytes)", size, baseLen)
	}
	if size > maxSize {
		return fmt.Errorf("size %d exceeds the maximum cart size %d; raise --max-size only for a cart with a mapper", size, maxSize)
	}
	return nil
}
