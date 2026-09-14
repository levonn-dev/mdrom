package bps

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
)

// minRepeat is the shortest run of one byte worth a TargetCopy instead of a TargetRead.
const minRepeat = 4

// Encode builds a patch turning source into target. It uses SourceRead for equal
// runs, TargetRead for differing bytes, and TargetCopy only as run-length encoding.
func Encode(source, target []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString(magic)
	putNumber(&buf, uint64(len(source)))
	putNumber(&buf, uint64(len(target)))
	putNumber(&buf, 0)
	e := encoder{buf: &buf, source: source, target: target}
	e.run()
	var crc [4]byte
	binary.LittleEndian.PutUint32(crc[:], crc32.ChecksumIEEE(source))
	buf.Write(crc[:])
	binary.LittleEndian.PutUint32(crc[:], crc32.ChecksumIEEE(target))
	buf.Write(crc[:])
	binary.LittleEndian.PutUint32(crc[:], crc32.ChecksumIEEE(buf.Bytes()))
	buf.Write(crc[:])
	return buf.Bytes()
}

type encoder struct {
	buf            *bytes.Buffer
	source, target []byte
	tgtRel         int // running TargetCopy pointer, as the decoder tracks it
}

func (e *encoder) same(i int) bool {
	return i < len(e.source) && e.source[i] == e.target[i]
}

func (e *encoder) run() {
	for i := 0; i < len(e.target); {
		j := i
		if e.same(i) {
			for j < len(e.target) && e.same(j) {
				j++
			}
			e.action(actSourceRead, j-i)
		} else {
			for j < len(e.target) && !e.same(j) {
				j++
			}
			e.differing(i, j)
		}
		i = j
	}
}

// differing emits target bytes [i, j), turning runs of one byte into TargetCopy.
func (e *encoder) differing(i, j int) {
	t := e.target
	pending := i
	for p := i; p < j; {
		r := p + 1
		for r < j && t[r] == t[p] {
			r++
		}
		if r-p >= minRepeat {
			e.targetRead(pending, p)
			e.targetRead(p, p+1)
			e.targetCopy(p, r-p-1)
			pending = r
		}
		p = r
	}
	e.targetRead(pending, j)
}

func (e *encoder) targetRead(from, to int) {
	if to <= from {
		return
	}
	e.action(actTargetRead, to-from)
	e.buf.Write(e.target[from:to])
}

// targetCopy copies length bytes starting at output position from, which must already be written.
func (e *encoder) targetCopy(from, length int) {
	e.action(actTargetCopy, length)
	delta := from - e.tgtRel
	var n uint64
	if delta < 0 {
		n = uint64(-delta)<<1 | 1
	} else {
		n = uint64(delta) << 1
	}
	putNumber(e.buf, n)
	e.tgtRel = from + length
}

func (e *encoder) action(act, length int) {
	putNumber(e.buf, uint64(length-1)<<2|uint64(act))
}
