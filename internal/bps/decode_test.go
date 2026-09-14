package bps

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
	"testing"
)

// build assembles a patch from a hand-written action body, with correct sizes and CRCs.
func build(source, target []byte, body func(*bytes.Buffer)) []byte {
	var buf bytes.Buffer
	buf.WriteString("BPS1")
	putNumber(&buf, uint64(len(source)))
	putNumber(&buf, uint64(len(target)))
	putNumber(&buf, 0)
	body(&buf)
	var crc [4]byte
	binary.LittleEndian.PutUint32(crc[:], crc32.ChecksumIEEE(source))
	buf.Write(crc[:])
	binary.LittleEndian.PutUint32(crc[:], crc32.ChecksumIEEE(target))
	buf.Write(crc[:])
	binary.LittleEndian.PutUint32(crc[:], crc32.ChecksumIEEE(buf.Bytes()))
	buf.Write(crc[:])
	return buf.Bytes()
}

func action(buf *bytes.Buffer, act int, length int) {
	putNumber(buf, uint64(length-1)<<2|uint64(act))
}

func TestDecodeAllFourActions(t *testing.T) {
	source := []byte("ABCDEFGH")
	target := []byte("ABEFGHEFGHxxxxxx")
	patch := build(source, target, func(b *bytes.Buffer) {
		action(b, actSourceRead, 2) // "AB"
		action(b, actSourceCopy, 4) // "EFGH" from source offset 4
		putNumber(b, 4<<1)          // relative +4
		action(b, actTargetCopy, 4) // "EFGH" again from target offset 2
		putNumber(b, 2<<1)          // relative +2
		action(b, actTargetRead, 1) // "x"
		b.WriteByte('x')
		action(b, actTargetCopy, 5) // run-length: copy from offset 10, one behind
		putNumber(b, 4<<1)          // target pointer was 6, move to 10
	})
	got, err := Decode(patch, source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, target) {
		t.Fatalf("Decode = %q, want %q", got, target)
	}
}

func TestDecodeRejectsBadMagic(t *testing.T) {
	patch := build(nil, nil, func(*bytes.Buffer) {})
	patch[0] = 'X'
	if _, err := Decode(patch, nil); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestDecodeRejectsHugeTarget(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("BPS1")
	putNumber(&buf, 0)                       // source size
	putNumber(&buf, uint64(maxTargetSize)+1) // target size: one past the cap
	putNumber(&buf, 0)                       // metadata size
	var crc [4]byte
	binary.LittleEndian.PutUint32(crc[:], crc32.ChecksumIEEE(nil)) // source CRC
	buf.Write(crc[:])
	binary.LittleEndian.PutUint32(crc[:], 0) // target CRC: never reached
	buf.Write(crc[:])
	binary.LittleEndian.PutUint32(crc[:], crc32.ChecksumIEEE(buf.Bytes())) // patch CRC
	buf.Write(crc[:])
	if _, err := Decode(buf.Bytes(), nil); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestDecodeRejectsCorruptPatch(t *testing.T) {
	source := []byte("hello")
	patch := build(source, source, func(b *bytes.Buffer) { action(b, actSourceRead, 5) })
	patch[len(patch)-13] ^= 0xFF
	if _, err := Decode(patch, source); !errors.Is(err, ErrPatchCRC) {
		t.Fatalf("err = %v, want ErrPatchCRC", err)
	}
}

func TestDecodeRejectsWrongSource(t *testing.T) {
	source := []byte("hello")
	patch := build(source, source, func(b *bytes.Buffer) { action(b, actSourceRead, 5) })
	if _, err := Decode(patch, []byte("jello")); !errors.Is(err, ErrSourceCRC) {
		t.Fatalf("err = %v, want ErrSourceCRC", err)
	}
}

func TestDecodeRejectsOverrun(t *testing.T) {
	source := []byte("hello")
	patch := build(source, source, func(b *bytes.Buffer) { action(b, actSourceRead, 6) })
	if _, err := Decode(patch, source); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestDecodeRejectsTargetCRC(t *testing.T) {
	source := []byte("hello")
	patch := build(source, source, func(b *bytes.Buffer) { action(b, actSourceRead, 5) })
	n := len(patch)
	for i := n - 8; i < n-4; i++ {
		patch[i] = 0xFF
	}
	binary.LittleEndian.PutUint32(patch[n-4:], crc32.ChecksumIEEE(patch[:n-4]))
	if _, err := Decode(patch, source); !errors.Is(err, ErrTargetCRC) {
		t.Fatalf("err = %v, want ErrTargetCRC", err)
	}
}

func TestDecodeRejectsCopyOffsetsPastEnd(t *testing.T) {
	source := []byte("hello")

	// SourceCopy of length 1 with a relative offset one past the end of source: no
	// overflow, just past the last valid byte.
	patch := build(source, []byte("x"), func(b *bytes.Buffer) {
		action(b, actSourceCopy, 1)
		putNumber(b, uint64(len(source))<<1)
	})
	if _, err := Decode(patch, source); !errors.Is(err, ErrFormat) {
		t.Fatalf("SourceCopy past end: err = %v, want ErrFormat", err)
	}

	// After a one-byte TargetRead, a TargetCopy of length 1 with relative offset +1
	// points at the byte currently being written (the pointer equals out).
	patch = build(source, []byte("xx"), func(b *bytes.Buffer) {
		action(b, actTargetRead, 1)
		b.WriteByte('x')
		action(b, actTargetCopy, 1)
		putNumber(b, 1<<1)
	})
	if _, err := Decode(patch, source); !errors.Is(err, ErrFormat) {
		t.Fatalf("TargetCopy pointer==out: err = %v, want ErrFormat", err)
	}
}

func TestDecodeRejectsOverflowingCopyOffsets(t *testing.T) {
	source := []byte("hello")
	target := []byte("hello")

	// Test SourceCopy with MaxInt64 offset
	patch := build(source, target, func(b *bytes.Buffer) {
		action(b, actSourceCopy, 1)
		putNumber(b, uint64(math.MaxInt64)<<1)
	})
	_, err := Decode(patch, source)
	if err == nil || !errors.Is(err, ErrFormat) {
		t.Fatalf("SourceCopy MaxInt64 offset should return ErrFormat, got %v", err)
	}

	// Test TargetCopy with MaxInt64 offset
	patch = build(source, target, func(b *bytes.Buffer) {
		action(b, actTargetRead, 1)
		b.WriteByte('h')
		action(b, actTargetCopy, 1)
		putNumber(b, uint64(math.MaxInt64)<<1)
	})
	_, err = Decode(patch, source)
	if err == nil || !errors.Is(err, ErrFormat) {
		t.Fatalf("TargetCopy MaxInt64 offset should return ErrFormat, got %v", err)
	}
}
