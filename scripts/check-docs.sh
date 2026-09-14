#!/bin/sh
# Compiles every ```c and ```asm block in the guide with the cross compiler and checks
# that every relative .md link resolves. Blocks tagged "c nocheck" or "asm nocheck" are
# skipped. A fence may be indented, as inside a list item; its indentation is stripped from
# the block. Usage: scripts/check-docs.sh   (or: task docs:check)
set -u
cd "$(dirname "$0")/.." || exit 1
command -v m68k-linux-gnu-gcc >/dev/null 2>&1 \
  || { echo "check-docs: m68k-linux-gnu-gcc not found (sudo apt install gcc-m68k-linux-gnu)" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail=0
snippets=0
links=0
for page in docs/*.md docs/recipes/*.md; do
  [ -e "$page" ] || continue
  # One file per checked block, named by the line of its opening fence.
  awk -v out="$tmp" '
    /^[ \t]*```/ && !inblock {
      indent = $0; sub(/```.*$/, "", indent)
      lang = $0; sub(/^[ \t]*```/, "", lang)
      start = NR; inblock = 1; body = ""; next
    }
    /^[ \t]*```/ && inblock  {
      ext = ""
      if (lang == "c") ext = "c"; else if (lang == "asm") ext = "s"
      if (ext != "") { f = out "/" start "." ext; printf "%s", body > f; close(f) }
      inblock = 0; next
    }
    inblock { line = $0; if (index(line, indent) == 1) line = substr(line, length(indent) + 1); body = body line "\n" }
  ' "$page"
  for f in "$tmp"/*.c "$tmp"/*.s; do
    [ -e "$f" ] || continue
    line=${f##*/}; line=${line%.*}
    ok=1
    case $f in
      *.c) m68k-linux-gnu-gcc -m68000 -ffreestanding -nostdlib -fno-pic -Os -Wall -Wextra -Werror \
             -c -o /dev/null "$f" 2>"$tmp/err" || ok=0 ;;
      *.s) m68k-linux-gnu-gcc -m68000 -x assembler -c -o /dev/null "$f" 2>"$tmp/err" || ok=0 ;;
    esac
    if [ "$ok" -eq 0 ]; then
      echo "$page:$line: snippet does not compile (line numbers below are relative to the fence)" >&2
      sed "s|^$f:|  |" "$tmp/err" >&2
      fail=1
    fi
    snippets=$((snippets + 1))
    rm -f "$f"
  done
  # Relative .md links, resolved against the page's directory; anchors are dropped.
  # Read from a file, not a pipe: a while loop in a pipeline could not update fail or links.
  grep -o '](\([^)#]*\.md\)\(#[^)]*\)\?)' "$page" | sed 's/^](//; s/[#)].*$//' > "$tmp/links"
  while read -r target; do
    case $target in http://*|https://*) continue ;; esac
    if [ ! -f "$(dirname "$page")/$target" ]; then
      echo "$page: link target $target does not exist" >&2
      fail=1
    fi
    links=$((links + 1))
  done < "$tmp/links"
done
[ "$fail" -eq 0 ] || exit 1
echo "check-docs: $snippets snippets compiled, $links links checked"
