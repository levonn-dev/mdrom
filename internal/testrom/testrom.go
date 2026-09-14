// Package testrom builds synthetic ROM images for tests.
package testrom

import "encoding/binary"

// Synth builds a size-byte image with a valid header, a reset vector into a
// bra.s self-loop at 0x200 so the image boots, no filler runs after the header,
// and three NOPs at 0x300 so the elfimg testdata hook can be applied.
func Synth(size int) []byte {
	img := make([]byte, size)
	for i := range img {
		img[i] = byte(1 + (i*7)%253) // never 0x00 or 0xFF
	}
	binary.BigEndian.PutUint32(img[0x000:], 0x00FFFE00) // initial stack pointer, top of work RAM
	binary.BigEndian.PutUint32(img[0x004:], 0x00000200) // initial program counter
	copy(img[0x200:], []byte{0x60, 0xFE})               // bra.s to itself
	copy(img[0x100:], "SEGA GENESIS    ")
	copy(img[0x110:], "(C)TEST 2026.SEP")
	copy(img[0x120:], "SYNTHETIC TEST ROM")
	for i := 0x120 + 18; i < 0x150; i++ {
		img[i] = ' '
	}
	copy(img[0x150:], "SYNTHETIC TEST ROM")
	for i := 0x150 + 18; i < 0x180; i++ {
		img[i] = ' '
	}
	copy(img[0x180:], "GM 00000000-00")
	binary.BigEndian.PutUint32(img[0x1A0:], 0)
	binary.BigEndian.PutUint32(img[0x1A4:], uint32(size-1))
	binary.BigEndian.PutUint32(img[0x1A8:], 0xFF0000)
	binary.BigEndian.PutUint32(img[0x1AC:], 0xFFFFFF)
	copy(img[0x1F0:], "U               ")
	copy(img[0x300:], []byte{0x4E, 0x71, 0x4E, 0x71, 0x4E, 0x71})
	var sum uint16
	for i := 0x200; i+1 < len(img); i += 2 {
		sum += binary.BigEndian.Uint16(img[i:])
	}
	binary.BigEndian.PutUint16(img[0x18E:], sum)
	return img
}
