package rom_test

import (
	"encoding/binary"
	"testing"

	"mdrom/internal/rom"
	"mdrom/internal/testrom"
)

func TestParseHeaderSynthetic(t *testing.T) {
	img := testrom.Synth(0x100000)
	h, err := rom.ParseHeader(img)
	if err != nil {
		t.Fatal(err)
	}
	if h.System != "SEGA GENESIS" || h.DomesticTitle != "SYNTHETIC TEST ROM" || h.Serial != "GM 00000000-00" {
		t.Errorf("text fields = %q %q %q", h.System, h.DomesticTitle, h.Serial)
	}
	if h.ROMStart != 0 || h.ROMEnd != 0x0FFFFF || h.RAMStart != 0xFF0000 || h.RAMEnd != 0xFFFFFF {
		t.Errorf("address fields = %#x %#x %#x %#x", h.ROMStart, h.ROMEnd, h.RAMStart, h.RAMEnd)
	}
	if h.HasSRAM {
		t.Error("synthetic image declares no SRAM")
	}
	if h.Region != "U" {
		t.Errorf("region = %q", h.Region)
	}
	if h.Checksum != rom.Checksum(img) {
		t.Errorf("stored checksum %#x != computed %#x", h.Checksum, rom.Checksum(img))
	}
}

func TestParseHeaderRejectsShortImage(t *testing.T) {
	if _, err := rom.ParseHeader(make([]byte, 0x100)); err == nil {
		t.Fatal("expected an error for an image shorter than the header")
	}
}

func TestChecksumSumsWordsAfterHeader(t *testing.T) {
	img := make([]byte, 0x204)
	img[0x100] = 0xFF // header bytes must not count
	copy(img[0x200:], []byte{0x12, 0x34, 0x56, 0x78})
	if got := rom.Checksum(img); got != 0x68AC {
		t.Fatalf("Checksum = %#x, want 0x68AC", got)
	}
}

func TestFixHeaderSetsEndAndChecksum(t *testing.T) {
	img := testrom.Synth(0x100000)
	img[0x18E], img[0x18F] = 0, 0
	img = append(img, make([]byte, 0x100000)...) // pretend expansion with zeros
	rom.FixHeader(img)
	if end := binary.BigEndian.Uint32(img[0x1A4:]); end != 0x1FFFFF {
		t.Errorf("ROM end = %#x, want 0x1FFFFF", end)
	}
	if stored := binary.BigEndian.Uint16(img[0x18E:]); stored != rom.Checksum(img) {
		t.Errorf("stored checksum %#x != computed %#x", stored, rom.Checksum(img))
	}
}

func TestParseHeaderReadsSRAM(t *testing.T) {
	img := testrom.Synth(0x100000)
	copy(img[0x1B0:], []byte{'R', 'A', 0xF8, 0x20})
	binary.BigEndian.PutUint32(img[0x1B4:], 0x200001)
	binary.BigEndian.PutUint32(img[0x1B8:], 0x203FFF)
	h, err := rom.ParseHeader(img)
	if err != nil {
		t.Fatal(err)
	}
	if !h.HasSRAM || h.SRAMType != 0xF8 || h.SRAMStart != 0x200001 || h.SRAMEnd != 0x203FFF {
		t.Errorf("sram = %v type %#x %#x-%#x, want backup RAM 0xF8 at 0x200001-0x203FFF", h.HasSRAM, h.SRAMType, h.SRAMStart, h.SRAMEnd)
	}
}
