// Command mdrom measures Mega Drive ROM images, injects linked code into them, and builds BPS patches.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
)

const usage = `usage: mdrom <command> [flags]

commands:
  init    <rom> [flags]                  generate a CMake patch project beside the ROM
  info    <rom>                          print header fields and checksum state
  free    <rom> [flags]                  report free space; --ld writes a linker MEMORY fragment
  inject  --base --elf --out [flags]     write ELF sections into a ROM image and fix its header
  bps     create --base --target --out   build a BPS patch
  bps     apply  --base --patch  --out   apply a BPS patch
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// errUsage marks an error whose explanation (flag descriptions, printed by
// parseFlags) is already on stderr; run reports it as exit code 2 with no
// "mdrom:" prefix instead of printing the sentinel itself.
var errUsage = errors.New("usage")

// run dispatches a command line and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "init":
		err = cmdInit(args[1:], stdout, stderr)
	case "info":
		err = cmdInfo(args[1:], stdout, stderr)
	case "free":
		err = cmdFree(args[1:], stdout, stderr)
	case "inject":
		err = cmdInject(args[1:], stdout, stderr)
	case "bps":
		err = cmdBPS(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		if errors.Is(err, errUsage) {
			return 2
		}
		fmt.Fprintln(stderr, "mdrom:", err)
		return 1
	}
	return 0
}

// newFlagSet returns a FlagSet that discards its own error output; parseOnce
// and parseFlags print flag descriptions themselves, to the stderr they are given.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parseOnce parses args into fs. On -h/--help it prints usage followed by fs's
// flag descriptions to stderr and returns errUsage; on any other parse error
// it returns that error unchanged.
func parseOnce(fs *flag.FlagSet, args []string, usage string, stderr io.Writer) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, usage)
			fs.SetOutput(stderr)
			fs.PrintDefaults()
			return errUsage
		}
		return err
	}
	return nil
}

// parseFlags parses args into fs and requires exactly positionals leftover
// arguments. On -h/--help it prints usage followed by fs's flag descriptions
// to stderr and returns errUsage; on any other parse error it returns that
// error; otherwise it checks the positional count, using usage when there are
// too few and unexpectedArgErr when there are too many.
func parseFlags(fs *flag.FlagSet, args []string, positionals int, usage string, stderr io.Writer) error {
	if err := parseOnce(fs, args, usage, stderr); err != nil {
		return err
	}
	if fs.NArg() > positionals {
		return unexpectedArgErr(fs, positionals)
	}
	if fs.NArg() < positionals {
		return errors.New(usage)
	}
	return nil
}

// unexpectedArgErr names the first argument beyond the number a command accepts.
func unexpectedArgErr(fs *flag.FlagSet, positionals int) error {
	return fmt.Errorf("unexpected argument %q", fs.Arg(positionals))
}

// parseSize accepts decimal or 0x-prefixed byte counts.
func parseSize(s string) (int, error) {
	n, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	if n <= 0 {
		return 0, errors.New("size must be positive")
	}
	return int(n), nil
}

// requireFlags returns an error naming the first empty value in name/value pairs, in the order given.
func requireFlags(cmd string, pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			return fmt.Errorf("%s: %s is required", cmd, pairs[i])
		}
	}
	return nil
}
