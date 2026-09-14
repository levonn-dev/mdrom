package elfimg

import (
	"bytes"
	"strings"
	"testing"
)

func find(t *testing.T, list []Section, name string) Section {
	t.Helper()
	for _, s := range list {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("section %s not found in %+v", name, list)
	return Section{}
}

func TestReadPlacedFixture(t *testing.T) {
	res, err := Read("testdata/poc.elf")
	if err != nil {
		t.Fatal(err)
	}
	text := find(t, res.Sections, ".text")
	wantText := []byte{0x30, 0x2F, 0x00, 0x06, 0xD0, 0x40, 0xD0, 0x6F, 0x00, 0x06, 0x52, 0x40, 0x4E, 0x75, 0x4E, 0x71}
	if text.Addr != 0x100000 || !bytes.Equal(text.Data, wantText) {
		t.Errorf(".text = %#x %x, want 0x100000 %x", text.Addr, text.Data, wantText)
	}
	hook := find(t, res.Sections, ".hook_000300")
	if hook.Addr != 0x300 || !bytes.Equal(hook.Data, []byte{0x4E, 0xB9, 0x00, 0x10, 0x00, 0x00}) {
		t.Errorf(".hook_000300 = %#x %x", hook.Addr, hook.Data)
	}
	if len(res.Sections) != 2 {
		t.Errorf("Sections = %d entries, want 2 (.text and the hook)", len(res.Sections))
	}
	orig := find(t, res.Origs, ".orig_000300")
	if orig.Addr != 0x300 || !bytes.Equal(orig.Data, []byte{0x4E, 0x71, 0x4E, 0x71, 0x4E, 0x71}) {
		t.Errorf(".orig_000300 = %#x %x", orig.Addr, orig.Data)
	}
	if len(res.NoBits) != 0 {
		t.Errorf("NoBits = %+v, want none", res.NoBits)
	}
}

func TestReadRejectsMisplacedHook(t *testing.T) {
	_, err := Read("testdata/misplaced.elf")
	if err == nil || !strings.Contains(err.Error(), "0x100010") || !strings.Contains(err.Error(), "--section-start") {
		t.Fatalf("err = %v, want a misplaced-hook error naming 0x100010 and the fix", err)
	}
}

func TestTaggedAddr(t *testing.T) {
	if got, err := taggedAddr(".hook_0051F2"); err != nil || got != 0x51F2 {
		t.Errorf("taggedAddr(.hook_0051F2) = %#x, %v", got, err)
	}
	for _, bad := range []string{".hook_300", ".orig_0000300", ".hook_00030G", ".hook_"} {
		if _, err := taggedAddr(bad); err == nil {
			t.Errorf("taggedAddr(%q) accepted a malformed name", bad)
		}
	}
}

func TestReadMissingFile(t *testing.T) {
	if _, err := Read("testdata/nope.elf"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestReadRejectsNonM68kELF(t *testing.T) {
	_, err := Read("testdata/host.elf")
	if err == nil || !strings.Contains(err.Error(), "not a 32-bit m68k ELF") {
		t.Fatalf("err = %v, want a not-a-32-bit-m68k-ELF error", err)
	}
}

func TestReadRejectsNonAllocHook(t *testing.T) {
	_, err := Read("testdata/noalloc.elf")
	if err == nil || !strings.Contains(err.Error(), `"ax"`) {
		t.Fatalf("err = %v, want an error naming the \"ax\" fix", err)
	}
}

func TestReadRejectsNoBitsHook(t *testing.T) {
	_, err := Read("testdata/nobits.elf")
	if err == nil || !strings.Contains(err.Error(), "SHT_NOBITS") {
		t.Fatalf("err = %v, want an error naming SHT_NOBITS", err)
	}
}

func TestReadMapsDataLoadAddressThroughSegment(t *testing.T) {
	res, err := Read("testdata/atdata.elf")
	if err != nil {
		t.Fatal(err)
	}
	data := find(t, res.Sections, ".data")
	if data.Addr < 0x100000 || data.Addr >= 0x100100 {
		t.Errorf(".data Addr = %#x, want [0x100000, 0x100100)", data.Addr)
	}
	wantData := []byte{0, 1, 0, 2, 0, 3, 0, 4}
	if !bytes.Equal(data.Data, wantData) {
		t.Errorf(".data Data = %x, want %x", data.Data, wantData)
	}
	bss := find(t, res.NoBits, ".bss")
	if bss.Addr < 0xFF0000 {
		t.Errorf(".bss Addr = %#x, want >= 0xFF0000", bss.Addr)
	}
	if bss.Data != nil {
		t.Errorf(".bss Data = %x, want nil", bss.Data)
	}
}

func TestUnverifiedPairsHooksWithOrigs(t *testing.T) {
	res, err := Read("testdata/poc.elf")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := res.Unverified(); err != nil || len(got) != 0 {
		t.Errorf("poc.elf: Unverified() = %v, %v, want none", got, err)
	}
	res, err = Read("testdata/unpaired.elf")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := res.Unverified(); err != nil || len(got) != 1 || got[0] != ".hook_000300" {
		t.Errorf("unpaired.elf: Unverified() = %v, %v, want [.hook_000300]", got, err)
	}
}

func TestUnverifiedRejectsOrphanAndMismatchedOrigs(t *testing.T) {
	for elf, want := range map[string]string{
		"testdata/orphanorig.elf": ".orig_000400 has no .hook_000400 twin",
		"testdata/shortorig.elf":  ".hook_000300 is 6 bytes but .orig_000300 is 4",
	} {
		res, err := Read(elf)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := res.Unverified(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", elf, err, want)
		}
	}
}
