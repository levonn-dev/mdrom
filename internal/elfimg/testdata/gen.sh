#!/bin/sh
# Rebuilds every ELF fixture under this directory from src/ with the m68k cross
# toolchain (and the host compiler for host.elf). Run from any directory.
#
#   poc.elf              hook placed at 0x300, plus .text
#   misplaced.elf        hook left as an orphan section (not at 0x300)
#   noalloc.elf          hook section declared without "ax"
#   buildid.elf          poc.elf plus hooks.o, linked with a build-id note
#   hookonly.elf         hook section alone, no other code
#   hookonly_buildid.elf hookonly.elf linked with a build-id note
#   gap.elf              poc.elf plus hooks.o, linked with .text at 0x1000
#                        instead of 0x100000, to land in an interior gap
#   nobits.elf           a .hook_ section declared as bss (SHT_NOBITS)
#   unpaired.elf         hook section with no .orig_ twin
#   shortorig.elf        hook section whose .orig_ twin is shorter than it
#   orphanorig.elf       poc hook pair plus an .orig_ section with no hook
#   atdata.elf           AT() fixture: .data loaded in ROM, run from RAM, plus
#                        a .bss variable
#   host.elf             a native, non-m68k ELF
set -e
cd "$(dirname "$0")"
CC=m68k-linux-gnu-gcc
CFLAGS="-m68000 -ffreestanding -nostdlib -fno-pic -fomit-frame-pointer -Os -Wall -Wextra"
LDFLAGS="-m68000 -nostdlib -static -Wl,--build-id=none -Wl,-z,noexecstack -Wl,--entry=0 -Wl,-Ttext=0x100000"
LDFLAGS_BUILDID="-m68000 -nostdlib -static -Wl,--build-id=sha1 -Wl,-z,noexecstack -Wl,--entry=0 -Wl,-Ttext=0x100000"
LDFLAGS_GAP="-m68000 -nostdlib -static -Wl,--build-id=none -Wl,-z,noexecstack -Wl,--entry=0 -Wl,-Ttext=0x1000"
LDFLAGS_SCRIPT="-m68000 -nostdlib -static -Wl,--build-id=none -Wl,-z,noexecstack -Wl,--entry=0 -Wl,-T,src/at.ld"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
$CC $CFLAGS -c src/poc.c -o "$tmp/poc.o"
$CC -m68000 -c src/hooks.s -o "$tmp/hooks.o"
$CC -m68000 -c src/hooks_noalloc.s -o "$tmp/hooks_noalloc.o"
$CC -m68000 -c src/hooks_only.s -o "$tmp/hooks_only.o"
$CC -m68000 -c src/hooks_nobits.s -o "$tmp/hooks_nobits.o"
$CC -m68000 -c src/hooks_unpaired.s -o "$tmp/hooks_unpaired.o"
$CC -m68000 -c src/hooks_shortorig.s -o "$tmp/hooks_shortorig.o"
$CC -m68000 -c src/hooks_orphan.s -o "$tmp/hooks_orphan.o"
$CC $CFLAGS -c src/atdata.c -o "$tmp/atdata.o"
$CC $LDFLAGS -Wl,--section-start=.hook_000300=0x300 -o poc.elf "$tmp/poc.o" "$tmp/hooks.o"
$CC $LDFLAGS -o misplaced.elf "$tmp/poc.o" "$tmp/hooks.o"
$CC $LDFLAGS -Wl,--section-start=.hook_000300=0x300 -o noalloc.elf "$tmp/poc.o" "$tmp/hooks_noalloc.o"
$CC $LDFLAGS_BUILDID -Wl,--section-start=.hook_000300=0x300 -o buildid.elf "$tmp/poc.o" "$tmp/hooks.o"
$CC $LDFLAGS -Wl,--section-start=.hook_000300=0x300 -o hookonly.elf "$tmp/hooks_only.o"
$CC $LDFLAGS_BUILDID -Wl,--section-start=.hook_000300=0x300 -o hookonly_buildid.elf "$tmp/hooks_only.o"
$CC $LDFLAGS_GAP -Wl,--section-start=.hook_000300=0x300 -o gap.elf "$tmp/poc.o" "$tmp/hooks.o"
$CC $LDFLAGS -Wl,--section-start=.hook_000300=0x300 -o nobits.elf "$tmp/hooks_nobits.o"
$CC $LDFLAGS -Wl,--section-start=.hook_000300=0x300 -o unpaired.elf "$tmp/hooks_unpaired.o"
$CC $LDFLAGS -Wl,--section-start=.hook_000300=0x300 -o shortorig.elf "$tmp/hooks_shortorig.o"
$CC $LDFLAGS -Wl,--section-start=.hook_000300=0x300 -o orphanorig.elf "$tmp/hooks_orphan.o"
$CC $LDFLAGS_SCRIPT -o atdata.elf "$tmp/atdata.o"
gcc -nostdlib -static -Wl,--build-id=none -o host.elf src/host.c
