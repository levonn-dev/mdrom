#!/bin/sh
# Builds tests/rt_test.c against src/rt/div.c with the host compiler and runs it.
# Usage: check-rt.sh OUTDIR
set -eu
[ $# -eq 1 ] || { echo "usage: check-rt.sh OUTDIR" >&2; exit 2; }
dir=$(cd "$(dirname "$0")/.." && pwd)
out=$1
mkdir -p "$out"
gcc -std=c99 -O2 -Wall -Wextra -o "$out/rt_test" "$dir/tests/rt_test.c" "$dir/src/rt/div.c"
"$out/rt_test"
