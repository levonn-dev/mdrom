package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mdrom/internal/rom"
	"mdrom/internal/scaffold"
)

const initUsage = "usage: mdrom init <rom> [--name NAME] [--size N]"

func cmdInit(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("init")
	name := fs.String("name", "", "CMake project name and game symbol file prefix (default: the ROM's directory name)")
	var size int
	fs.Func("size", "target image size in bytes (default: the next power of two above the ROM, at most 0x400000)", func(v string) error {
		n, err := parseSize(v)
		size = n
		return err
	})
	// The flag package stops at the first positional argument, so parse, take the
	// ROM path, then parse whatever followed it.
	if err := parseOnce(fs, args, initUsage, stderr); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New(initUsage)
	}
	romPath := rest[0]
	if err := parseFlags(fs, rest[1:], 0, initUsage, stderr); err != nil {
		return err
	}
	img, err := os.ReadFile(romPath)
	if err != nil {
		return err
	}
	p, warnings, err := initParams(romPath, img, *name, size)
	if err != nil {
		return err
	}
	warnNonPowerOfTwo(p.Size, stderr)
	for _, w := range warnings {
		fmt.Fprintf(stderr, "%s\n", w)
	}
	fmt.Fprintf(stdout, "size: %d (0x%X); the ROM is %d (0x%X)\n", p.Size, p.Size, len(img), len(img))
	dir := filepath.Dir(romPath)
	files, err := scaffold.Write(dir, p)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	for _, f := range files {
		fmt.Fprintf(stdout, "wrote %s\n", filepath.Join(dir, filepath.FromSlash(f.Path)))
	}
	next := "cmake --preset m68k && cmake --build --preset m68k"
	if dir != "." {
		next = "cd " + dir + " && " + next
	}
	fmt.Fprintf(stdout, "next: %s\n", next)
	return nil
}

// initParams derives the template values from the ROM image and the flags. The
// warnings are stderr messages (the no-expansion note, then the SRAM warning), each
// without a trailing newline, for cmdInit to print once it has decided init will run.
func initParams(romPath string, img []byte, name string, size int) (scaffold.Params, []string, error) {
	var p scaffold.Params
	p.ROMFile = filepath.Base(romPath)
	if strings.ContainsAny(p.ROMFile, "\"\\;$") {
		return p, nil, fmt.Errorf("ROM filename %q contains a character CMake cannot quote (one of \" \\ ; $)", p.ROMFile)
	}
	h, err := rom.ParseHeader(img)
	if err != nil {
		return p, nil, err
	}
	// init has no --max-size flag, so give actionable advice instead of rom.ValidateSize's --max-size mention.
	if len(img) > rom.MaxCartSize {
		return p, nil, fmt.Errorf("image is %d bytes, larger than the %d-byte cartridge window; init targets carts without a mapper", len(img), rom.MaxCartSize)
	}
	if size == 0 {
		size = defaultSize(len(img))
	}
	if err := rom.ValidateSize(len(img), size, rom.MaxCartSize); err != nil {
		return p, nil, err
	}
	p.Size = size
	var warnings []string
	note, err := noExpansionNote(img, size)
	if err != nil {
		return p, nil, err
	}
	if note != "" {
		warnings = append(warnings, note)
	}
	if w := sramWarning(h, size); w != "" {
		warnings = append(warnings, w)
	}
	p.Entry = binary.BigEndian.Uint32(img[4:8])
	if p.Entry%2 != 0 || p.Entry < uint32(rom.HeaderSize) || p.Entry >= uint32(len(img)) {
		return p, nil, fmt.Errorf("reset vector 0x%08X is not a code address inside the ROM", p.Entry)
	}
	// The 68000 drives 24 address lines, so the top byte is ignored; work RAM mirrors
	// through 0xE00000-0xFFFFFF, and 0 is the top of RAM after the first push.
	sp := binary.BigEndian.Uint32(img[0:4])
	a := sp & 0xFFFFFF
	if sp%2 != 0 || (a != 0 && a < 0xE00000) {
		return p, nil, fmt.Errorf("initial stack pointer 0x%08X is not an even address in work RAM (0xE00000-0xFFFFFF)", sp)
	}
	p.SHA1 = rom.SHA1Hex(img)
	if name == "" {
		abs, err := filepath.Abs(romPath)
		if err != nil {
			return p, nil, err
		}
		name = sanitizeName(filepath.Base(filepath.Dir(abs)))
	}
	if err := validName(name); err != nil {
		return p, nil, err
	}
	p.Name = name
	return p, warnings, nil
}

// sramWarning warns when size reaches the header's SRAM window: an image that large is
// shadowed by SRAM in emulators and on hardware, so a mapperless cart cannot expose both.
func sramWarning(h rom.Header, size int) string {
	if !h.HasSRAM || uint32(size) <= h.SRAMStart {
		return ""
	}
	return fmt.Sprintf("warning: size %d (0x%X) reaches the SRAM range the header maps at 0x%06X-0x%06X; a cart without a mapper cannot expose both, pass --size to stay below it", size, size, h.SRAMStart, h.SRAMEnd)
}

// noExpansionNote scans for tail padding when size adds no room beyond the base image,
// since that turns "no expansion" from a nonissue into either a warning or a hard failure.
func noExpansionNote(img []byte, size int) (string, error) {
	if size > len(img) {
		return "", nil
	}
	regions, err := rom.Scan(img, size, 256)
	if err != nil {
		return "", err
	}
	usable, ok := rom.Usable(regions)
	if !ok {
		reason := "pass a larger --size"
		if len(img) >= rom.MaxCartSize {
			reason = "init targets carts without a mapper"
		}
		return "", fmt.Errorf("no free space: size %d adds no expansion and the ROM has no tail padding; %s", size, reason)
	}
	return fmt.Sprintf("warning: no expansion; the patch must fit in the %d-byte tail padding at 0x%06X", usable.Len(), usable.Start), nil
}

// defaultSize is the smallest power of two above n, capped at the cartridge window.
func defaultSize(n int) int {
	if n >= rom.MaxCartSize {
		return n
	}
	size := 1
	for size <= n {
		size <<= 1
	}
	if size > rom.MaxCartSize {
		return rom.MaxCartSize
	}
	return size
}

// isNameRune reports whether r may appear in a project name: [a-z0-9_].
func isNameRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_'
}

// sanitizeName lowercases s and replaces every character outside [a-z0-9_] with '_'.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if isNameRune(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// validName requires the form [a-z_][a-z0-9_]*, which CMake and the C guard both accept.
func validName(s string) error {
	if s == "" || s[0] >= '0' && s[0] <= '9' {
		return fmt.Errorf("project name %q must start with a letter; pass --name", s)
	}
	for _, r := range s {
		if !isNameRune(r) {
			return fmt.Errorf("project name %q may contain only a-z, 0-9, and _; pass --name", s)
		}
	}
	return nil
}
