#!/bin/sh
# Fails if the disassembly uses an instruction or addressing mode the 68000 lacks; our
# code is built with -m68000, so a hit means a prebuilt object such as libgcc crept in.
set -u
elf=$1
dis=$(m68k-linux-gnu-objdump -d "$elf") || exit 1
lines=$(printf '%s\n' "$dis" | grep -E '^\s+[0-9a-f]+:' | sed 's/<[^>]*>//g')
if [ -z "$lines" ]; then
  echo "check-68000: no instructions disassembled from $elf" >&2
  exit 1
fi
bad=$(printf '%s\n' "$lines" | grep -E \
  '\b(bsrl|bral|b(cc|cs|eq|ge|gt|hi|le|ls|lt|mi|ne|pl|vc|vs)l|divul|divsl|mulul|mulsl|extbl|bf[a-z]+|linkl|rtd|chk2|cmp2|pack|unpk|callm|rtm|trap(cc|cs|eq|f|ge|gt|hi|le|ls|lt|mi|ne|pl|t|vc|vs)|bkpt|movec|moves|cas|cas2)\b|:[248]\)|\[')
if [ -n "$bad" ]; then
  echo "check-68000: 68020-only instructions in $elf:" >&2
  echo "$bad" >&2
  exit 1
fi
echo "check-68000: $elf is 68000-clean"
