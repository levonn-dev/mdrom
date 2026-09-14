package bps

import (
	"bytes"
	"math/rand"
	"testing"
)

func roundTrip(t *testing.T, name string, source, target []byte) []byte {
	t.Helper()
	patch := Encode(source, target)
	got, err := Decode(patch, source)
	if err != nil {
		t.Fatalf("%s: decode: %v", name, err)
	}
	if !bytes.Equal(got, target) {
		t.Fatalf("%s: round trip mismatch", name)
	}
	return patch
}

func TestEncodeRoundTrips(t *testing.T) {
	src := []byte("the quick brown fox jumps over the lazy dog")
	rng := rand.New(rand.NewSource(1))
	random := make([]byte, 4096)
	rng.Read(random)
	cases := map[string]struct{ source, target []byte }{
		"identical":        {src, src},
		"empty":            {nil, nil},
		"all different":    {src, bytes.ToUpper(src)},
		"target shorter":   {src, src[:10]},
		"target longer":    {src, append(append([]byte{}, src...), " and more"...)},
		"from nothing":     {nil, src},
		"random":           {random, append(append([]byte{}, random[:2000]...), random[1000:]...)},
		"long repeat":      {src, append(append([]byte{}, src...), bytes.Repeat([]byte{0xFF}, 100000)...)},
		"repeat then diff": {src, append(bytes.Repeat([]byte{0}, 50), 'z')},
	}
	for name, c := range cases {
		roundTrip(t, name, c.source, c.target)
	}
}

func TestEncodeCompressesPadding(t *testing.T) {
	source := make([]byte, 1<<20)
	for i := range source {
		source[i] = byte(i)
	}
	target := append(append([]byte{}, source...), bytes.Repeat([]byte{0xFF}, 1<<20)...)
	patch := roundTrip(t, "padding", source, target)
	if len(patch) > 64 {
		t.Fatalf("patch for 1MB of padding is %d bytes; run-length TargetCopy is not working", len(patch))
	}
}

func TestEncodeSmallChangeIsSmall(t *testing.T) {
	source := make([]byte, 1<<20)
	for i := range source {
		source[i] = byte(i * 7)
	}
	target := append([]byte{}, source...)
	copy(target[0x300:], []byte{0x4E, 0xB9, 0x00, 0x10, 0x00, 0x00})
	patch := roundTrip(t, "small change", source, target)
	if len(patch) > 64 {
		t.Fatalf("patch for a 6-byte change is %d bytes", len(patch))
	}
}
