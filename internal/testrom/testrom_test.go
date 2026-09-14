package testrom

import (
	"encoding/binary"
	"testing"
)

func TestSynthHasResetVectorAndSelfLoop(t *testing.T) {
	img := Synth(0x100000)
	if sp := binary.BigEndian.Uint32(img[0:4]); sp != 0x00FFFE00 {
		t.Errorf("initial SP = 0x%08X, want 0x00FFFE00", sp)
	}
	if pc := binary.BigEndian.Uint32(img[4:8]); pc != 0x200 {
		t.Errorf("initial PC = 0x%08X, want 0x00000200", pc)
	}
	if img[0x200] != 0x60 || img[0x201] != 0xFE {
		t.Errorf("bytes at 0x200 = %02X %02X, want 60 FE (bra.s to self)", img[0x200], img[0x201])
	}
	var sum uint16
	for i := 0x200; i+1 < len(img); i += 2 {
		sum += binary.BigEndian.Uint16(img[i:])
	}
	if stored := binary.BigEndian.Uint16(img[0x18E:]); stored != sum {
		t.Errorf("stored checksum 0x%04X, computed 0x%04X", stored, sum)
	}
}
