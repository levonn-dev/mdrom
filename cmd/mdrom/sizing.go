package main

import (
	"flag"
	"fmt"
	"io"

	"mdrom/internal/rom"
)

// sizing holds the flags that decide how large the image is and which space counts as free.
type sizing struct {
	size     int
	maxSize  int
	minRun   int
	interior bool
}

func (s *sizing) addFlags(fs *flag.FlagSet) {
	s.maxSize = rom.MaxCartSize
	fs.Func("size", "image size in bytes; the base is padded up to it (default: the base size)", func(v string) error {
		n, err := parseSize(v)
		s.size = n
		return err
	})
	fs.Func("max-size", "largest allowed image; raise only for a cart with a mapper (default 0x400000)", func(v string) error {
		n, err := parseSize(v)
		s.maxSize = n
		return err
	})
	fs.IntVar(&s.minRun, "min", 256, "shortest interior filler run to report")
	fs.BoolVar(&s.interior, "interior", false, "treat interior filler runs as usable free space")
}

// resolve fills in the default size and validates it against the base image.
func (s *sizing) resolve(baseLen int, stderr io.Writer) error {
	if s.size == 0 {
		s.size = baseLen
	}
	if err := rom.ValidateSize(baseLen, s.size, s.maxSize); err != nil {
		return err
	}
	warnNonPowerOfTwo(s.size, stderr)
	return nil
}

// warnNonPowerOfTwo prints nothing when size is already a power of two; some emulators
// and flash carts assume one, so a mismatched size is worth flagging even though it works.
func warnNonPowerOfTwo(size int, stderr io.Writer) {
	if rom.IsPowerOfTwo(size) {
		return
	}
	fmt.Fprintf(stderr, "warning: size %d is not a power of two; some emulators and flash carts assume it\n", size)
}

// scan returns every region in img plus the merged tail-and-expansion region. ok reports
// whether a usable region exists; err is only a real scan error. A caller that needs
// somewhere to put code must check ok itself (see cmdFree); cmdInject tolerates !ok
// because hook sections need no free space.
func (s *sizing) scan(img []byte) (scanned []rom.Region, usable rom.Region, ok bool, err error) {
	scanned, err = rom.Scan(img, s.size, s.minRun)
	if err != nil {
		return nil, rom.Region{}, false, err
	}
	usable, ok = rom.Usable(scanned)
	return scanned, usable, ok, nil
}

// allowed returns the regions the linker may target: usable first when ok, then
// interior gaps with --interior.
func (s *sizing) allowed(scanned []rom.Region, usable rom.Region, ok bool) []rom.Region {
	var out []rom.Region
	if ok {
		out = append(out, usable)
	}
	if s.interior {
		for _, r := range scanned {
			if r.Kind == rom.Interior {
				out = append(out, r)
			}
		}
	}
	return out
}
