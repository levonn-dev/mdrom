package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"mdrom/internal/elfimg"
	"mdrom/internal/ldfrag"
	"mdrom/internal/rom"
)

func cmdInject(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("inject")
	var s sizing
	s.addFlags(fs)
	base := fs.String("base", "", "clean ROM image")
	elfPath := fs.String("elf", "", "linked ELF whose sections are written into the image")
	out := fs.String("out", "", "patched ROM to write")
	allowOverwrite := fs.Bool("allow-overwrite", false, "warn instead of failing when a section overlaps non-free bytes")
	allowUnverified := fs.Bool("allow-unverified-hook", false, "warn instead of failing when a hook has no .orig_ twin")
	expect := fs.String("expect-sha1", "", "fail unless the base ROM has this SHA-1")
	if err := parseFlags(fs, args, 0, "usage: mdrom inject --base ROM --elf ELF --out ROM [flags]", stderr); err != nil {
		return err
	}
	if err := requireFlags("inject", "--base", *base, "--elf", *elfPath, "--out", *out); err != nil {
		return err
	}
	img, err := os.ReadFile(*base)
	if err != nil {
		return err
	}
	if *expect != "" && !strings.EqualFold(rom.SHA1Hex(img), *expect) {
		return fmt.Errorf("base ROM SHA-1 is %s, expected %s", rom.SHA1Hex(img), *expect)
	}
	if err := s.resolve(len(img), stderr); err != nil {
		return err
	}
	scanned, usable, spaceOK, err := s.scan(img)
	if err != nil {
		return err
	}
	allowed := s.allowed(scanned, usable, spaceOK)
	res, err := elfimg.Read(*elfPath)
	if err != nil {
		return err
	}
	unverified, err := res.Unverified()
	if err != nil {
		return err
	}
	for _, name := range unverified {
		msg := fmt.Sprintf("%s has no .orig_%s twin recording the bytes it overwrites", name, strings.TrimPrefix(name, ".hook_"))
		if !*allowUnverified {
			return errors.New(msg + "; pass --allow-unverified-hook to inject it unchecked")
		}
		fmt.Fprintln(stderr, "warning:", msg)
	}
	for _, o := range res.Origs {
		start, end := int(o.Addr), int(o.Addr)+len(o.Data)
		if end > len(img) {
			return fmt.Errorf("%s: 0x%06X-0x%06X is beyond the %d-byte base image", o.Name, start, end, len(img))
		}
		if !bytes.Equal(img[start:end], o.Data) {
			return fmt.Errorf("%s: base ROM has %X at 0x%06X, expected %X", o.Name, img[start:end], start, o.Data)
		}
	}
	padded, err := rom.Pad(img, s.size)
	if err != nil {
		return err
	}
	sections := append([]elfimg.Section(nil), res.Sections...)
	sort.Slice(sections, func(i, j int) bool { return sections[i].Addr < sections[j].Addr })
	type row struct {
		name   string
		start  int
		size   int
		region string
	}
	var rows []row
	usedEnd := usable.Start
	for _, sec := range sections {
		isHook := strings.HasPrefix(sec.Name, ".hook_")
		if !isHook && len(allowed) == 0 && !*allowOverwrite {
			return fmt.Errorf("%s at 0x%06X-0x%06X needs free space but the image has no tail padding and no expansion; pass --size to expand it%s", sec.Name, sec.Addr, sec.Addr+uint64(len(sec.Data)), buildIDHint(sec.Name))
		}
		if sec.Addr+uint64(len(sec.Data)) > uint64(len(padded)) {
			return fmt.Errorf("%s at 0x%06X-0x%06X is outside the %d-byte image%s", sec.Name, sec.Addr, sec.Addr+uint64(len(sec.Data)), len(padded), buildIDHint(sec.Name))
		}
		start, end := int(sec.Addr), int(sec.Addr)+len(sec.Data)
		region := "hook"
		if !isHook {
			r, clobber, ok := findRegion(allowed, start, end)
			if !ok {
				msg := fmt.Sprintf("%s at 0x%06X-0x%06X would overwrite non-free byte 0x%06X", sec.Name, start, end, clobber)
				if !*allowOverwrite {
					advice := "; pass --allow-overwrite if that is intended"
					if !spaceOK {
						advice = "; pass --size to expand the image, or --allow-overwrite if that is intended"
					}
					return errors.New(msg + advice)
				}
				fmt.Fprintln(stderr, "warning:", msg)
				region = "overwrite"
			} else {
				region = ldfrag.RegionName(r, usable)
				if region == "ROM_FREE" && end > usedEnd {
					usedEnd = end
				}
			}
		}
		copy(padded[start:], sec.Data)
		rows = append(rows, row{sec.Name, start, len(sec.Data), region})
	}
	rom.FixHeader(padded)
	if err := os.WriteFile(*out, padded, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%-20s %-9s %-8s %s\n", "section", "address", "size", "region")
	for _, r := range rows {
		fmt.Fprintf(stdout, "%-20s 0x%06X  %-8d %s\n", r.name, r.start, r.size, r.region)
	}
	for _, nb := range res.NoBits {
		fmt.Fprintf(stdout, "bss: %s at 0x%06X (not written)\n", nb.Name, nb.Addr)
	}
	fmt.Fprintf(stdout, "rom end: 0x%06X  checksum: 0x%04X\n", len(padded)-1, rom.Checksum(padded))
	if spaceOK {
		fmt.Fprintf(stdout, "ROM_FREE unused: %d of %d bytes\n", usable.End-usedEnd, usable.Len())
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return nil
}

// buildIDHint names the fix for a build-id note that has nowhere to go: it appears
// whenever the linker adds a .note.gnu.build-id section that the image cannot fit.
func buildIDHint(name string) string {
	if name == ".note.gnu.build-id" {
		return "; link with -Wl,--build-id=none"
	}
	return ""
}

// findRegion reports whether [start, end) lies entirely inside one allowed region.
// When it does, ok is true and region names it (clobber is unused). Otherwise ok is
// false and clobber is the first byte of the range that is not free.
func findRegion(allowed []rom.Region, start, end int) (region rom.Region, clobber int, ok bool) {
	for _, r := range allowed {
		if start >= r.Start && start < r.End {
			if end <= r.End {
				return r, 0, true
			}
			return rom.Region{}, r.End, false
		}
	}
	return rom.Region{}, start, false
}
