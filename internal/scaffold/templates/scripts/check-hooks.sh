#!/bin/sh
# Every .hook_XXXXXX needs an .orig_XXXXXX of the same size, and the HOOKS list must name
# exactly the hook sections linked; ld says nothing about either, and mdrom inject never sees HOOKS.
# Usage: check-hooks.sh ELF HOOK...
set -u
elf=$1; shift
sections=$(readelf -S -W "$elf") || exit 1
sizes=$(printf '%s\n' "$sections" | sed -n 's/^ *\[ *[0-9]*\] \(\.\(hook\|orig\)_[0-9A-Fa-f]*\) *[A-Z]* *[0-9a-f]* *[0-9a-f]* *\([0-9a-f]*\).*/\1 \3/p')
size_of() { printf '%s\n' "$sizes" | awk -v n="$1" '$1 == n { print $2 }'; }
fail=0
for hook in "$@"; do
  h=$(size_of ".hook_$hook"); o=$(size_of ".orig_$hook")
  if [ -z "$h" ]; then echo "check-hooks: HOOKS lists $hook but no .hook_$hook section was linked" >&2; fail=1; continue; fi
  if [ -z "$o" ]; then echo "check-hooks: .hook_$hook has no .orig_$hook section" >&2; fail=1
  elif [ "$h" != "$o" ]; then echo "check-hooks: .hook_$hook is 0x$h bytes but .orig_$hook is 0x$o" >&2; fail=1; fi
done
for name in $(printf '%s\n' "$sizes" | awk '{ print $1 }'); do
  case $name in .hook_*) hook=${name#.hook_} ;; *) continue ;; esac
  case " $* " in *" $hook "*) ;; *) echo "check-hooks: $name is linked but $hook is not in HOOKS" >&2; fail=1 ;; esac
done
for name in $(printf '%s\n' "$sizes" | awk '{ print $1 }'); do
  case $name in .orig_*) hook=${name#.orig_} ;; *) continue ;; esac
  [ -n "$(size_of ".hook_$hook")" ] || { echo "check-hooks: $name has no .hook_$hook section" >&2; fail=1; }
done
[ "$fail" -eq 0 ] || exit 1
echo "check-hooks: $# hooks paired with their .orig_ sections"
