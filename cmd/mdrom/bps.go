package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"mdrom/internal/bps"
	"mdrom/internal/rom"
)

func cmdBPS(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: mdrom bps create|apply [flags]")
	}
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(stderr, "usage: mdrom bps create|apply [flags]")
		return errUsage
	}
	switch args[0] {
	case "create":
		return bpsCreate(args[1:], stdout, stderr)
	case "apply":
		return bpsApply(args[1:], stdout, stderr)
	}
	return fmt.Errorf("unknown bps command %q; use create|apply", args[0])
}

func bpsCreate(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("bps create")
	base := fs.String("base", "", "clean ROM image")
	target := fs.String("target", "", "patched ROM image")
	out := fs.String("out", "", "BPS file to write")
	if err := parseFlags(fs, args, 0, "usage: mdrom bps create --base ROM --target ROM --out BPS", stderr); err != nil {
		return err
	}
	if err := requireFlags("bps create", "--base", *base, "--target", *target, "--out", *out); err != nil {
		return err
	}
	src, err := os.ReadFile(*base)
	if err != nil {
		return err
	}
	tgt, err := os.ReadFile(*target)
	if err != nil {
		return err
	}
	patch := bps.Encode(src, tgt)
	check, err := bps.Decode(patch, src)
	if err != nil {
		return fmt.Errorf("self-check failed: %w", err)
	}
	if !bytes.Equal(check, tgt) {
		return errors.New("self-check failed: applying the new patch does not reproduce the target")
	}
	if err := os.WriteFile(*out, patch, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s (%d bytes, self-check passed)\n", *out, len(patch))
	return nil
}

func bpsApply(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("bps apply")
	base := fs.String("base", "", "clean ROM image")
	patchPath := fs.String("patch", "", "BPS file to apply")
	out := fs.String("out", "", "patched ROM to write")
	if err := parseFlags(fs, args, 0, "usage: mdrom bps apply --base ROM --patch BPS --out ROM", stderr); err != nil {
		return err
	}
	if err := requireFlags("bps apply", "--base", *base, "--patch", *patchPath, "--out", *out); err != nil {
		return err
	}
	src, err := os.ReadFile(*base)
	if err != nil {
		return err
	}
	patch, err := os.ReadFile(*patchPath)
	if err != nil {
		return err
	}
	tgt, err := bps.Decode(patch, src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, tgt, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s (%d bytes, sha1 %s)\n", *out, len(tgt), rom.SHA1Hex(tgt))
	return nil
}
