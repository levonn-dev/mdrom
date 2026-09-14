package rom_test

import (
	"reflect"
	"testing"

	"mdrom/internal/rom"
	"mdrom/internal/testrom"
)

// gappy returns a 0x1000-byte image with: 128 zeros at 0x400, 0xFF from the odd
// address 0x601 up to 0x901, and 256 zeros reaching the end.
func gappy() []byte {
	img := testrom.Synth(0x1000)
	for i := 0x400; i < 0x480; i++ {
		img[i] = 0x00
	}
	for i := 0x601; i < 0x901; i++ {
		img[i] = 0xFF
	}
	for i := 0xF00; i < 0x1000; i++ {
		img[i] = 0x00
	}
	return img
}

func TestScanClassifiesRuns(t *testing.T) {
	got, err := rom.Scan(gappy(), 0x2000, 256)
	if err != nil {
		t.Fatal(err)
	}
	want := []rom.Region{
		{Start: 0x602, End: 0x901, Fill: 0xFF, Kind: rom.Interior},
		{Start: 0xF00, End: 0x1000, Fill: 0x00, Kind: rom.Tail},
		{Start: 0x1000, End: 0x2000, Fill: 0xFF, Kind: rom.Expansion},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v\nwant %+v", got, want)
	}
}

func TestScanKeepsShortRunsWithLowerMin(t *testing.T) {
	got, err := rom.Scan(gappy(), 0x1000, 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Start != 0x400 || got[0].End != 0x480 {
		t.Fatalf("Scan = %+v, want the 128-byte run first and no expansion", got)
	}
}

func TestScanIgnoresHeaderAndNoExpansionWhenSizeEqualsImage(t *testing.T) {
	img := testrom.Synth(0x1000) // header holds zero bytes but nothing after 0x200 is filler
	got, err := rom.Scan(img, 0x1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("Scan = %+v, want no regions", got)
	}
}

func TestScanRejectsShortImage(t *testing.T) {
	if _, err := rom.Scan(make([]byte, 0x100), 0x100, 1); err == nil {
		t.Fatal("expected an error for an image shorter than the header")
	}
}

func TestUsableMergesTailAndExpansion(t *testing.T) {
	regions, _ := rom.Scan(gappy(), 0x2000, 256)
	u, ok := rom.Usable(regions)
	if !ok {
		t.Fatal("expected a usable region")
	}
	if u.Start != 0xF00 || u.End != 0x2000 || u.Kind != rom.Expansion {
		t.Fatalf("Usable = %+v", u)
	}
}

func TestUsableTailOnly(t *testing.T) {
	regions, _ := rom.Scan(gappy(), 0x1000, 256)
	u, ok := rom.Usable(regions)
	if !ok || u.Start != 0xF00 || u.End != 0x1000 || u.Kind != rom.Tail {
		t.Fatalf("Usable = %+v ok=%v", u, ok)
	}
}

func TestUsableNone(t *testing.T) {
	if _, ok := rom.Usable(nil); ok {
		t.Fatal("expected no usable region")
	}
}

func TestRegionLen(t *testing.T) {
	r := rom.Region{Start: 0x100, End: 0x200}
	if r.Len() != 0x100 {
		t.Fatalf("Len = %#x", r.Len())
	}
}
