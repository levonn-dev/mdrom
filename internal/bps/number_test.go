package bps

import (
	"bytes"
	"testing"
)

func TestNumberRoundTrip(t *testing.T) {
	for _, n := range []uint64{0, 1, 127, 128, 129, 16383, 16384, 1 << 20, 1 << 32, 1<<40 + 5} {
		var buf bytes.Buffer
		putNumber(&buf, n)
		r := &reader{data: buf.Bytes()}
		got := r.number()
		if r.err != nil || got != n || r.pos != buf.Len() {
			t.Errorf("number %d: got %d err %v pos %d/%d", n, got, r.err, r.pos, buf.Len())
		}
	}
}

func TestNumberKnownEncodings(t *testing.T) {
	var buf bytes.Buffer
	putNumber(&buf, 0)
	putNumber(&buf, 127)
	putNumber(&buf, 128)
	want := []byte{0x80, 0xFF, 0x00, 0x80}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("encodings = %x, want %x", buf.Bytes(), want)
	}
}

func TestNumberTruncated(t *testing.T) {
	r := &reader{data: []byte{0x00}}
	r.number()
	if r.err == nil {
		t.Fatal("expected an error on a truncated number")
	}
}
