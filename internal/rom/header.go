package rom

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Header holds the fields of the cartridge header at 0x100.
type Header struct {
	System        string
	Copyright     string
	DomesticTitle string
	OverseasTitle string
	Serial        string
	Checksum      uint16
	ROMStart      uint32
	ROMEnd        uint32
	RAMStart      uint32
	RAMEnd        uint32
	HasSRAM       bool
	SRAMType      byte
	SRAMStart     uint32
	SRAMEnd       uint32
	Region        string
}

const (
	offSystem    = 0x100
	offCopyright = 0x110
	offDomestic  = 0x120
	offOverseas  = 0x150
	offSerial    = 0x180
	offChecksum  = 0x18E
	offROMStart  = 0x1A0
	offROMEnd    = 0x1A4
	offRAMStart  = 0x1A8
	offRAMEnd    = 0x1AC
	offSRAM      = 0x1B0
	offRegion    = 0x1F0
)

func text(img []byte, off, n int) string {
	return strings.TrimRight(string(img[off:off+n]), " \x00")
}

// ParseHeader reads the cartridge header. img must be at least HeaderSize bytes.
func ParseHeader(img []byte) (Header, error) {
	if len(img) < HeaderSize {
		return Header{}, fmt.Errorf("image is %d bytes, shorter than the %d-byte header", len(img), HeaderSize)
	}
	be := binary.BigEndian
	h := Header{
		System:        text(img, offSystem, 16),
		Copyright:     text(img, offCopyright, 16),
		DomesticTitle: text(img, offDomestic, 48),
		OverseasTitle: text(img, offOverseas, 48),
		Serial:        text(img, offSerial, 14),
		Checksum:      be.Uint16(img[offChecksum:]),
		ROMStart:      be.Uint32(img[offROMStart:]),
		ROMEnd:        be.Uint32(img[offROMEnd:]),
		RAMStart:      be.Uint32(img[offRAMStart:]),
		RAMEnd:        be.Uint32(img[offRAMEnd:]),
		Region:        text(img, offRegion, 3),
	}
	if string(img[offSRAM:offSRAM+2]) == "RA" {
		h.HasSRAM = true
		h.SRAMType = img[offSRAM+2]
		h.SRAMStart = be.Uint32(img[offSRAM+4:])
		h.SRAMEnd = be.Uint32(img[offSRAM+8:])
	}
	return h, nil
}

// Checksum computes the header checksum: the 16-bit sum of big-endian words after the header.
func Checksum(img []byte) uint16 {
	var sum uint16
	for i := HeaderSize; i+1 < len(img); i += 2 {
		sum += binary.BigEndian.Uint16(img[i:])
	}
	return sum
}

// FixHeader sets the ROM end address and checksum to match img. img must be at least HeaderSize bytes.
func FixHeader(img []byte) {
	binary.BigEndian.PutUint32(img[offROMEnd:], uint32(len(img)-1))
	binary.BigEndian.PutUint16(img[offChecksum:], Checksum(img))
}
