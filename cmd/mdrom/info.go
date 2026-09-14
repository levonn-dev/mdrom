package main

import (
	"fmt"
	"io"
	"os"

	"mdrom/internal/rom"
)

func cmdInfo(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("info")
	if err := parseFlags(fs, args, 1, "usage: mdrom info <rom>", stderr); err != nil {
		return err
	}
	romPath := fs.Arg(0)
	img, err := os.ReadFile(romPath)
	if err != nil {
		return err
	}
	h, err := rom.ParseHeader(img)
	if err != nil {
		return err
	}
	computed := rom.Checksum(img)
	state := "OK"
	if computed != h.Checksum {
		state = "MISMATCH"
	}
	sram := "none"
	if h.HasSRAM {
		sram = fmt.Sprintf("type 0x%02X at 0x%06X-0x%06X", h.SRAMType, h.SRAMStart, h.SRAMEnd)
	}
	fmt.Fprintf(stdout, "file:      %s\n", fs.Arg(0))
	fmt.Fprintf(stdout, "size:      %d (0x%X)\n", len(img), len(img))
	fmt.Fprintf(stdout, "sha1:      %s\n", rom.SHA1Hex(img))
	fmt.Fprintf(stdout, "system:    %s\n", h.System)
	fmt.Fprintf(stdout, "copyright: %s\n", h.Copyright)
	fmt.Fprintf(stdout, "domestic:  %s\n", h.DomesticTitle)
	fmt.Fprintf(stdout, "overseas:  %s\n", h.OverseasTitle)
	fmt.Fprintf(stdout, "serial:    %s\n", h.Serial)
	fmt.Fprintf(stdout, "rom:       0x%06X-0x%06X\n", h.ROMStart, h.ROMEnd)
	fmt.Fprintf(stdout, "ram:       0x%06X-0x%06X\n", h.RAMStart, h.RAMEnd)
	fmt.Fprintf(stdout, "sram:      %s\n", sram)
	fmt.Fprintf(stdout, "region:    %s\n", h.Region)
	fmt.Fprintf(stdout, "checksum:  stored 0x%04X computed 0x%04X %s\n", h.Checksum, computed, state)
	return nil
}
