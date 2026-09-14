package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"mdrom/internal/ldfrag"
	"mdrom/internal/rom"
)

const freeUsage = "usage: mdrom free <rom> [--size N] [--max-size N] [--min N] [--interior] [--ld FILE]"

func cmdFree(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("free")
	var s sizing
	s.addFlags(fs)
	ld := fs.String("ld", "", "write a linker MEMORY fragment to this file")
	// The flag package stops at the first positional argument, so parse, take the
	// ROM path, then parse whatever followed it.
	if err := parseOnce(fs, args, freeUsage, stderr); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New(freeUsage)
	}
	romPath := rest[0]
	if err := parseFlags(fs, rest[1:], 0, freeUsage, stderr); err != nil {
		return err
	}

	img, err := os.ReadFile(romPath)
	if err != nil {
		return err
	}
	if err := s.resolve(len(img), stderr); err != nil {
		return err
	}
	scanned, usable, ok, err := s.scan(img)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("no usable free space: no tail padding and no expansion; pass --size to expand the image")
	}
	fmt.Fprintf(stdout, "%-9s %-9s %-9s %-5s %s\n", "start", "end", "length", "fill", "kind")
	interiorRuns, interiorBytes := 0, 0
	for _, r := range scanned {
		fmt.Fprintf(stdout, "0x%06X  0x%06X  0x%06X  0x%02X  %s\n", r.Start, r.End, r.Len(), r.Fill, r.Kind)
		if r.Kind == rom.Interior {
			interiorRuns++
			interiorBytes += r.Len()
		}
	}
	fmt.Fprintf(stdout, "usable: %d bytes at 0x%06X (tail + expansion)\n", usable.Len(), usable.Start)
	note := "not used without --interior"
	if s.interior {
		note = "included with --interior"
	}
	fmt.Fprintf(stdout, "interior candidates: %d bytes in %d runs (%s)\n", interiorBytes, interiorRuns, note)
	fmt.Fprintf(stdout, "headroom: %d bytes below max size %d\n", s.maxSize-s.size, s.maxSize)
	if *ld == "" {
		return nil
	}
	var buf bytes.Buffer
	if err := ldfrag.Write(&buf, usable, s.allowed(scanned, usable, true)[1:]); err != nil {
		return err
	}
	if err := os.WriteFile(*ld, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s\n", *ld)
	return nil
}
