// Package bps encodes and decodes BPS patches.
package bps

import "bytes"

// putNumber appends n in the BPS variable-length encoding.
func putNumber(buf *bytes.Buffer, n uint64) {
	for {
		x := byte(n & 0x7F)
		n >>= 7
		if n == 0 {
			buf.WriteByte(x | 0x80)
			return
		}
		buf.WriteByte(x)
		n--
	}
}

// reader walks patch bytes, recording the first error instead of returning one per call.
type reader struct {
	data []byte
	pos  int
	err  error
}

func (r *reader) number() uint64 {
	var n, shift uint64 = 0, 1
	for {
		if r.pos >= len(r.data) {
			r.err = ErrFormat
			return 0
		}
		x := r.data[r.pos]
		r.pos++
		n += uint64(x&0x7F) * shift
		if x&0x80 != 0 {
			return n
		}
		shift <<= 7
		n += shift
	}
}
