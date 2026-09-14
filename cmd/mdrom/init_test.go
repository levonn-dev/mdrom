package main

import (
	"bytes"
	"encoding/binary"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mdrom/internal/rom"
	"mdrom/internal/testrom"
)

// initFiles is every path mdrom init writes, relative to the project root.
var initFiles = []string{
	"CMakeLists.txt", "CMakePresets.json",
	"cmake/m68k-toolchain.cmake", "cmake/mdrom.cmake",
	"link/rom.ld", "hooks/hooks.s", "src/patch.c",
	"src/rt/div.c", "src/rt/mul.s", "tests/rt_test.c",
	"scripts/check-68000.sh", "scripts/check-hooks.sh", "scripts/check-rt.sh",
}

// writeProjectROM writes a synthetic 1MB ROM as <tmp>/<dirName>/base.md and returns its path.
func writeProjectROM(t *testing.T, dirName string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), dirName)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "base.md")
	mustWrite(t, path, testrom.Synth(0x100000))
	return path
}

func TestInitWritesTree(t *testing.T) {
	romPath := writeProjectROM(t, "my-game")
	dir := filepath.Dir(romPath)
	code, stdout, stderr := exec("init", romPath)
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	for _, rel := range append(initFiles, "game/my_game.h", "game/my_game.ld") {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("%s: %v", rel, err)
		}
		if !strings.Contains(stdout, "wrote "+filepath.Join(dir, rel)+"\n") {
			t.Errorf("stdout lacks the wrote line for %s", rel)
		}
	}
	cmake := string(mustRead(t, filepath.Join(dir, "CMakeLists.txt")))
	for _, want := range []string{
		"project(my_game C ASM)",
		`"${CMAKE_SOURCE_DIR}/base.md"`,
		"set(CLEAN_ROM_SHA1 " + rom.SHA1Hex(testrom.Synth(0x100000)) + ")",
		"set(ROM_SIZE 2097152) # 0x200000",
	} {
		if !strings.Contains(cmake, want) {
			t.Errorf("CMakeLists.txt lacks %q", want)
		}
	}
	hooks := string(mustRead(t, filepath.Join(dir, "hooks", "hooks.s")))
	for _, want := range []string{".long   0x00000200", "jmp     0x200 "} {
		if !strings.Contains(hooks, want) {
			t.Errorf("hooks.s lacks %q", want)
		}
	}
	if h := string(mustRead(t, filepath.Join(dir, "game", "my_game.h"))); !strings.Contains(h, "#ifndef MY_GAME_H") {
		t.Errorf("my_game.h lacks its guard:\n%s", h)
	}
	for _, s := range []string{"check-68000.sh", "check-hooks.sh", "check-rt.sh"} {
		info, err := os.Stat(filepath.Join(dir, "scripts", s))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Errorf("scripts/%s is not executable", s)
		}
	}
	if want := "next: cd " + dir + " && cmake --preset m68k && cmake --build --preset m68k\n"; !strings.HasSuffix(stdout, want) {
		t.Errorf("stdout does not end with %q:\n%s", want, stdout)
	}
	if want := "size: 2097152 (0x200000); the ROM is 1048576 (0x100000)\n"; !strings.HasPrefix(stdout, want) {
		t.Errorf("stdout does not start with %q:\n%s", want, stdout)
	}
}

func TestInitNextLineWithoutCd(t *testing.T) {
	romPath := writeProjectROM(t, "cwdgame")
	t.Chdir(filepath.Dir(romPath))
	code, stdout, stderr := exec("init", "base.md")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "wrote CMakeLists.txt\n") {
		t.Errorf("stdout lacks a bare wrote line:\n%s", stdout)
	}
	if !strings.HasSuffix(stdout, "next: cmake --preset m68k && cmake --build --preset m68k\n") {
		t.Errorf("stdout does not end with the bare next line:\n%s", stdout)
	}
	if _, err := os.Stat("game/cwdgame.h"); err != nil {
		t.Errorf("default name from the current directory: %v", err)
	}
}

func TestInitNameFlag(t *testing.T) {
	romPath := writeProjectROM(t, "proj")
	code, _, stderr := exec("init", romPath, "--name", "oltec")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(romPath), "game", "oltec.ld")); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{"Bad-Name", "9lives"} {
		romPath := writeProjectROM(t, "proj")
		code, _, stderr := exec("init", romPath, "--name", bad)
		if code != 1 || !strings.Contains(stderr, "pass --name") {
			t.Errorf("--name %q: code %d stderr %q", bad, code, stderr)
		}
	}
}

func TestInitDefaultNameNeedsALetter(t *testing.T) {
	romPath := writeProjectROM(t, "2048")
	code, _, stderr := exec("init", romPath)
	if code != 1 || !strings.Contains(stderr, `project name "2048" must start with a letter; pass --name`) {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
}

func TestInitSizeFlag(t *testing.T) {
	romPath := writeProjectROM(t, "proj")
	code, _, stderr := exec("init", romPath, "--size", "0x400000")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if cmake := string(mustRead(t, filepath.Join(filepath.Dir(romPath), "CMakeLists.txt"))); !strings.Contains(cmake, "set(ROM_SIZE 4194304) # 0x400000") {
		t.Error("CMakeLists.txt lacks the 4MB size")
	}
	for size, want := range map[string]string{
		"0x80000":  "smaller than the base image",
		"0x200001": "is odd",
		"0x800000": "exceeds the maximum cart size",
	} {
		romPath := writeProjectROM(t, "proj")
		code, _, stderr := exec("init", romPath, "--size", size)
		if code != 1 || !strings.Contains(stderr, want) {
			t.Errorf("--size %s: code %d stderr %q, want %q", size, code, stderr, want)
		}
	}
	romPath = writeProjectROM(t, "proj")
	if code, _, stderr := exec("init", romPath, "--size", "0x180000"); code != 0 || !strings.Contains(stderr, "not a power of two") {
		t.Errorf("--size 0x180000: code %d stderr %q", code, stderr)
	}
}

func TestInitRejectsBadResetVector(t *testing.T) {
	for name, entry := range map[string]uint32{"odd": 0x201, "past end": 0x100000, "in header": 0x100} {
		romPath := writeProjectROM(t, "proj")
		img := mustRead(t, romPath)
		binary.BigEndian.PutUint32(img[4:], entry)
		mustWrite(t, romPath, img)
		code, _, stderr := exec("init", romPath)
		if code != 1 || !strings.Contains(stderr, "is not a code address inside the ROM") {
			t.Errorf("%s: code %d stderr %q", name, code, stderr)
		}
	}
}

func TestInitRejectsBadStackPointer(t *testing.T) {
	for name, sp := range map[string]uint32{
		"in ROM":           0x00001000,
		"odd":              0xFFFFFFF7,
		"above RAM mirror": 0x00D00000,
	} {
		romPath := writeProjectROM(t, "proj")
		img := mustRead(t, romPath)
		binary.BigEndian.PutUint32(img[0:], sp)
		mustWrite(t, romPath, img)
		code, _, stderr := exec("init", romPath)
		if code != 1 || !strings.Contains(stderr, "is not an even address in work RAM") {
			t.Errorf("%s: code %d stderr %q", name, code, stderr)
		}
	}
	for name, sp := range map[string]uint32{
		"wotes form": 0xFFFFFFF6,
		"top of RAM": 0x01000000,
		"zero":       0x00000000,
	} {
		romPath := writeProjectROM(t, "proj")
		img := mustRead(t, romPath)
		binary.BigEndian.PutUint32(img[0:], sp)
		mustWrite(t, romPath, img)
		code, _, stderr := exec("init", romPath)
		if code != 0 {
			t.Errorf("%s: code %d stderr %q", name, code, stderr)
		}
	}
}

// setSRAMHeader writes the "RA" backup-RAM fields ParseHeader reads: the signature and
// flag bytes, then the start and end addresses.
func setSRAMHeader(img []byte, start, end uint32) {
	copy(img[0x1B0:], []byte{'R', 'A', 0xF8, 0x20})
	binary.BigEndian.PutUint32(img[0x1B4:], start)
	binary.BigEndian.PutUint32(img[0x1B8:], end)
}

func TestInitWarnsWhenSizeReachesSRAM(t *testing.T) {
	romPath := writeProjectROM(t, "sram-game")
	img := mustRead(t, romPath)
	setSRAMHeader(img, 0x200001, 0x203FFF)
	mustWrite(t, romPath, img)
	if h, err := rom.ParseHeader(img); err != nil || !h.HasSRAM {
		t.Fatalf("test setup: HasSRAM = %v, err %v", h.HasSRAM, err)
	}
	code, _, stderr := exec("init", romPath, "--size", "0x400000")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if !strings.Contains(stderr, "reaches the SRAM range the header maps at 0x200001-0x203FFF") {
		t.Errorf("stderr lacks the SRAM warning:\n%s", stderr)
	}

	romPath = writeProjectROM(t, "sram-game-default")
	img = mustRead(t, romPath)
	setSRAMHeader(img, 0x200001, 0x203FFF)
	mustWrite(t, romPath, img)
	code, _, stderr = exec("init", romPath)
	if code != 0 {
		t.Fatalf("default size: code %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "SRAM") {
		t.Errorf("default size (below SRAMStart) should not warn about SRAM:\n%s", stderr)
	}
}

// TestInitWarningOrder pins the relative order of init's stderr warnings, since nothing
// else in the test suite checks it once more than one can fire on the same run.
func TestInitWarningOrder(t *testing.T) {
	romPath := writeProjectROM(t, "sram-order")
	img := mustRead(t, romPath)
	setSRAMHeader(img, 0x200001, 0x203FFF)
	mustWrite(t, romPath, img)
	code, _, stderr := exec("init", romPath, "--size", "0x300000")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	pow := strings.Index(stderr, "not a power of two")
	sram := strings.Index(stderr, "reaches the SRAM range")
	if pow < 0 || sram < 0 || pow >= sram {
		t.Errorf("want the power-of-two warning before the SRAM warning, got indexes %d, %d:\n%s", pow, sram, stderr)
	}

	// A 1MB ROM at --size 0x100000 (a power of two, so no power-of-two warning) with
	// both a usable tail and an SRAM window fires the no-expansion and SRAM warnings
	// together, so their order is checkable without the power-of-two warning in the mix.
	romPath = writeProjectROM(t, "tail-and-sram")
	img = mustRead(t, romPath)
	for i := len(img) - 0x100; i < len(img); i++ {
		img[i] = 0xFF
	}
	setSRAMHeader(img, 0x080001, 0x083FFF)
	mustWrite(t, romPath, img)
	code, _, stderr = exec("init", romPath, "--size", "0x100000")
	if code != 0 {
		t.Fatalf("tail+SRAM: code %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "not a power of two") {
		t.Errorf("0x100000 is a power of two; should not warn:\n%s", stderr)
	}
	noExp := strings.Index(stderr, "no expansion;")
	sram = strings.Index(stderr, "reaches the SRAM range")
	if noExp < 0 || sram < 0 || noExp >= sram {
		t.Errorf("want the no-expansion note before the SRAM warning, got indexes %d, %d:\n%s", noExp, sram, stderr)
	}
}

func TestSRAMWarning(t *testing.T) {
	h := rom.Header{HasSRAM: false, SRAMStart: 0x200000, SRAMEnd: 0x203FFF}
	if got := sramWarning(h, 0x400000); got != "" {
		t.Errorf("HasSRAM=false: got %q, want empty", got)
	}
	h = rom.Header{HasSRAM: true, SRAMStart: 0x200000, SRAMEnd: 0x203FFF}
	if got := sramWarning(h, 0x200000); got != "" {
		t.Errorf("size == SRAMStart: got %q, want empty", got)
	}
}

func TestInitRejectsFullCartWithoutTail(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	romPath := filepath.Join(dir, "base.md")
	mustWrite(t, romPath, testrom.Synth(rom.MaxCartSize))
	code, _, stderr := exec("init", romPath)
	if code != 1 || !strings.Contains(stderr, "adds no expansion and the ROM has no tail padding") || !strings.Contains(stderr, "without a mapper") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
}

func TestInitRejectsNoExpansionWithoutTail(t *testing.T) {
	romPath := writeProjectROM(t, "proj")
	code, _, stderr := exec("init", romPath, "--size", "0x100000")
	if code != 1 || !strings.Contains(stderr, "pass a larger --size") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
}

func TestInitWarnsNoExpansionWithTail(t *testing.T) {
	romPath := writeProjectROM(t, "proj")
	img := mustRead(t, romPath)
	for i := len(img) - 0x100; i < len(img); i++ {
		img[i] = 0xFF
	}
	mustWrite(t, romPath, img)
	code, _, stderr := exec("init", romPath, "--size", "0x100000")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if !strings.Contains(stderr, "no expansion; the patch must fit in the 256-byte tail padding at 0x0FFF00") {
		t.Errorf("stderr lacks the tail-padding warning:\n%s", stderr)
	}
}

func TestInitRejectsUnquotableROMName(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, `ba"se.md`)
	mustWrite(t, romPath, testrom.Synth(0x100000))
	code, _, stderr := exec("init", romPath)
	if code != 1 || !strings.Contains(stderr, "contains a character CMake cannot quote") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
}

func TestInitRejectsShortImage(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "base.md")
	mustWrite(t, romPath, make([]byte, 0x100))
	code, _, stderr := exec("init", romPath)
	if code != 1 || !strings.Contains(stderr, "shorter than the") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
}

func TestInitRefusesExistingFiles(t *testing.T) {
	romPath := writeProjectROM(t, "proj")
	dir := filepath.Dir(romPath)
	if code, _, stderr := exec("init", romPath); code != 0 {
		t.Fatalf("first init: code %d stderr %q", code, stderr)
	}
	marker := append(mustRead(t, filepath.Join(dir, "CMakeLists.txt")), []byte("# mine\n")...)
	mustWrite(t, filepath.Join(dir, "CMakeLists.txt"), marker)
	code, _, stderr := exec("init", romPath)
	if code != 1 || !strings.Contains(stderr, "init: "+filepath.Join(dir, "CMakeLists.txt")+" already exists") {
		t.Fatalf("second init: code %d stderr %q", code, stderr)
	}
	if !bytes.Equal(mustRead(t, filepath.Join(dir, "CMakeLists.txt")), marker) {
		t.Error("CMakeLists.txt was rewritten")
	}
}

func TestInitWritesNothingWhenOneFileExists(t *testing.T) {
	romPath := writeProjectROM(t, "proj")
	dir := filepath.Dir(romPath)
	if err := os.Mkdir(filepath.Join(dir, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "hooks", "hooks.s"), []byte("mine\n"))
	code, _, stderr := exec("init", romPath)
	if code != 1 || !strings.Contains(stderr, filepath.Join("hooks", "hooks.s")+" already exists") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	for _, rel := range append(initFiles, "game/proj.h", "game/proj.ld") {
		if rel == "hooks/hooks.s" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			t.Errorf("%s was written despite the refusal", rel)
		}
	}
}

// TestInitRejectsOversizeROM checks that a ROM larger than the cartridge window fails
// with advice that does not reference the nonexistent --max-size flag.
func TestInitRejectsOversizeROM(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	romPath := filepath.Join(dir, "base.md")
	mustWrite(t, romPath, testrom.Synth(rom.MaxCartSize+2))
	code, _, stderr := exec("init", romPath)
	if code != 1 || !strings.Contains(stderr, "larger than the") || !strings.Contains(stderr, "without a mapper") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
}

func TestInitArgs(t *testing.T) {
	romPath := writeProjectROM(t, "proj")
	if code, _, stderr := exec("init"); code != 1 || !strings.Contains(stderr, "usage: mdrom init") {
		t.Errorf("no args: code %d stderr %q", code, stderr)
	}
	if code, _, stderr := exec("init", romPath, "extra"); code != 1 || !strings.Contains(stderr, `unexpected argument "extra"`) {
		t.Errorf("extra arg: code %d stderr %q", code, stderr)
	}
	if code, _, stderr := exec("init", romPath, "--bogus"); code != 1 || !strings.Contains(stderr, "bogus") {
		t.Errorf("unknown flag: code %d stderr %q", code, stderr)
	}
	if code, _, stderr := exec("init", "-h"); code != 2 || !strings.Contains(stderr, "usage: mdrom init <rom> [--name NAME] [--size N]") || !strings.Contains(stderr, "-size") {
		t.Errorf("-h: code %d stderr %q", code, stderr)
	}
	if code, _, stderr := exec("init", filepath.Join(t.TempDir(), "missing.md")); code != 1 || !strings.Contains(stderr, "mdrom:") {
		t.Errorf("missing ROM: code %d stderr %q", code, stderr)
	}
}

func TestDefaultSize(t *testing.T) {
	for n, want := range map[int]int{
		0x080000: 0x100000,
		0x100000: 0x200000,
		0x180000: 0x200000,
		0x300000: 0x400000,
		0x400000: 0x400000,
	} {
		if got := defaultSize(n); got != want {
			t.Errorf("defaultSize(0x%X) = 0x%X, want 0x%X", n, got, want)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"my-game":     "my_game",
		"Wotes Oltec": "wotes_oltec",
		"D&D (USA)":   "d_d__usa_",
		"ok_name_9":   "ok_name_9",
		"über":        "_ber",
	} {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestInitBuilds generates a project from the synthetic ROM and builds it with
// the real toolchain. It needs the cross compiler, CMake, Ninja, and the host gcc.
func TestInitBuilds(t *testing.T) {
	for _, tool := range []string{"m68k-linux-gnu-gcc", "m68k-linux-gnu-objdump", "gcc", "readelf", "cmake", "ninja"} {
		if _, err := osexec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH; skipping the end-to-end build", tool)
		}
	}
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "proj")
	if err := os.Mkdir(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	romPath := filepath.Join(proj, "base.md")
	mustWrite(t, romPath, testrom.Synth(0x100000))
	bin := filepath.Join(tmp, "mdrom")
	runTool(t, "", "go", "build", "-o", bin, ".")
	if code, _, stderr := exec("init", romPath); code != 0 {
		t.Fatalf("init: code %d stderr %q", code, stderr)
	}
	runTool(t, proj, "cmake", "--preset", "m68k", "-DMDROM="+bin)
	runTool(t, proj, "cmake", "--build", "--preset", "m68k")
	patched := mustRead(t, filepath.Join(proj, "build", "m68k", "patched.md"))
	if entry := binary.BigEndian.Uint32(patched[4:8]); entry < 0x100000 || entry >= 0x200000 {
		t.Errorf("patched reset vector 0x%08X is not in the expansion", entry)
	}
	applied := filepath.Join(tmp, "applied.md")
	code, _, stderr := exec("bps", "apply", "--base", romPath, "--patch", filepath.Join(proj, "build", "m68k", "patch.bps"), "--out", applied)
	if code != 0 {
		t.Fatalf("bps apply: code %d stderr %q", code, stderr)
	}
	if !bytes.Equal(mustRead(t, applied), patched) {
		t.Error("applying patch.bps to the ROM does not reproduce patched.md")
	}
}

// runTool runs an external tool in dir and fails the test with its output when it exits non-zero.
func runTool(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := osexec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
