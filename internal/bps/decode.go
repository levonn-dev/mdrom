package bps

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

const magic = "BPS1"

const (
	actSourceRead = 0
	actTargetRead = 1
	actSourceCopy = 2
	actTargetCopy = 3
)

const footerSize = 12

const maxTargetSize = 64 << 20 // far above any cartridge; bounds a hostile patch's allocation

var (
	ErrFormat    = errors.New("bps: malformed patch")
	ErrSourceCRC = errors.New("bps: source checksum mismatch")
	ErrTargetCRC = errors.New("bps: target checksum mismatch")
	ErrPatchCRC  = errors.New("bps: patch checksum mismatch")
)

// Decode applies patch to source and verifies the patch, source, and target checksums.
func Decode(patch, source []byte) ([]byte, error) {
	if len(patch) < len(magic)+footerSize || string(patch[:4]) != magic {
		return nil, ErrFormat
	}
	footer := len(patch) - footerSize
	srcCRC := binary.LittleEndian.Uint32(patch[footer:])
	tgtCRC := binary.LittleEndian.Uint32(patch[footer+4:])
	patCRC := binary.LittleEndian.Uint32(patch[footer+8:])
	if crc32.ChecksumIEEE(patch[:footer+8]) != patCRC {
		return nil, ErrPatchCRC
	}
	if crc32.ChecksumIEEE(source) != srcCRC {
		return nil, ErrSourceCRC
	}
	r := &reader{data: patch[:footer], pos: len(magic)}
	srcSize, tgtSize, metaSize := r.number(), r.number(), r.number()
	if r.err != nil {
		return nil, r.err
	}
	if srcSize != uint64(len(source)) {
		return nil, fmt.Errorf("%w: patch expects a %d-byte source, got %d bytes", ErrFormat, srcSize, len(source))
	}
	if metaSize > uint64(footer-r.pos) || tgtSize > maxTargetSize {
		return nil, ErrFormat
	}
	r.pos += int(metaSize)
	target := make([]byte, tgtSize)
	var out, srcRel, tgtRel int
	for r.pos < footer {
		data := r.number()
		if r.err != nil {
			return nil, r.err
		}
		length := int(data>>2) + 1
		if out+length > len(target) {
			return nil, ErrFormat
		}
		switch data & 3 {
		case actSourceRead:
			if out+length > len(source) {
				return nil, ErrFormat
			}
			copy(target[out:], source[out:out+length])
		case actTargetRead:
			if r.pos+length > footer {
				return nil, ErrFormat
			}
			copy(target[out:], patch[r.pos:r.pos+length])
			r.pos += length
		case actSourceCopy:
			srcRel += relOffset(r.number())
			if r.err != nil || srcRel < 0 || length > len(source)-srcRel {
				return nil, ErrFormat
			}
			copy(target[out:], source[srcRel:srcRel+length])
			srcRel += length
		case actTargetCopy:
			tgtRel += relOffset(r.number())
			if r.err != nil || tgtRel < 0 || tgtRel >= out || length > len(target)-tgtRel {
				return nil, ErrFormat
			}
			for i := 0; i < length; i++ {
				target[out+i] = target[tgtRel+i]
			}
			tgtRel += length
		}
		out += length
	}
	if out != len(target) {
		return nil, ErrFormat
	}
	if crc32.ChecksumIEEE(target) != tgtCRC {
		return nil, ErrTargetCRC
	}
	return target, nil
}

// relOffset decodes a signed relative offset: low bit is the sign, the rest the magnitude.
func relOffset(n uint64) int {
	off := int(n >> 1)
	if n&1 != 0 {
		return -off
	}
	return off
}
