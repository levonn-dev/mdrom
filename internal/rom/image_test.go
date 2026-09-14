package rom

import (
	"bytes"
	"testing"
)

func TestSHA1Hex(t *testing.T) {
	got := SHA1Hex([]byte("abc"))
	want := "a9993e364706816aba3e25717850c26c9cd0d89d"
	if got != want {
		t.Fatalf("SHA1Hex = %s, want %s", got, want)
	}
}

func TestPadExtendsWithFF(t *testing.T) {
	out, err := Pad([]byte{1, 2, 3}, 6)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 2, 3, 0xFF, 0xFF, 0xFF}
	if !bytes.Equal(out, want) {
		t.Fatalf("Pad = %x, want %x", out, want)
	}
}

func TestPadDoesNotAliasInput(t *testing.T) {
	in := []byte{1, 2}
	out, _ := Pad(in, 2)
	out[0] = 9
	if in[0] != 1 {
		t.Fatal("Pad returned the input slice instead of a copy")
	}
}

func TestPadRefusesShrink(t *testing.T) {
	if _, err := Pad(make([]byte, 4), 2); err == nil {
		t.Fatal("expected an error when size is smaller than the image")
	}
}

func TestIsPowerOfTwo(t *testing.T) {
	for n, want := range map[int]bool{0: false, 1: true, 2: true, 3: false, 0x100000: true, 0x180000: false} {
		if got := IsPowerOfTwo(n); got != want {
			t.Errorf("IsPowerOfTwo(%#x) = %v, want %v", n, got, want)
		}
	}
}

func TestValidateSize(t *testing.T) {
	cases := []struct {
		base, size, max int
		ok              bool
	}{
		{0x100000, 0x200000, MaxCartSize, true},
		{0x100000, 0x100000, MaxCartSize, true},
		{0x100000, 0x080000, MaxCartSize, false},
		{0x100000, 0x800000, MaxCartSize, false},
		{0x100000, 0x800000, 0x800000, true},
		{0x100001, 0x100001, MaxCartSize, false},
		{0x100000, 0x100001, MaxCartSize, false},
	}
	for _, c := range cases {
		err := ValidateSize(c.base, c.size, c.max)
		if (err == nil) != c.ok {
			t.Errorf("ValidateSize(%#x, %#x, %#x) = %v, want ok=%v", c.base, c.size, c.max, err, c.ok)
		}
	}
}
