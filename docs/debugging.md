# Debugging

A GDB stub is code an emulator runs beside its own CPU core: it pauses that CPU on request and answers GDB's remote protocol with breakpoints, single-stepping, and reads and writes of registers and memory, against the emulated 68000 rather than the host. Connecting differs only in how GDB reaches the stub. This page connects to BlastEm over a pipe and MAME over TCP, works through one session, sets a watchpoint, gives a scripted check, and lists five failures with where to look first.

## What you need

- `gdb-multiarch`: a GDB build whose targets include `m68k`; plain `gdb` on a 64-bit host usually lacks it.
- An emulator with a GDB stub open: a pipe GDB spawns, or a TCP port it listens on.
- The build's `patch.elf` as the symbol file: `-g` gives it debug information without changing a ROM byte ([The build pipeline](build-pipeline.md)), so GDB resolves `patch_init`, `patch_ret`, and every C local by name.

## BlastEm

The pipe form spawns BlastEm over its own stdin and stdout, so no host or port needs naming:

```text
(gdb) target remote | blastem patched.md -D
```

`-D` starts BlastEm paused rather than running: nothing executes until GDB's first `continue`, so the session below finds the CPU stopped wherever the reset vector currently points, a patch's hook included.

Only a current build of BlastEm answers this correctly: Ubuntu's packaged `blastem`, version 0.6.3.4, does not speak to GDB 15, so a newer build ahead of it on the PATH is required. Under WSLg, that build also needs its configuration's `gl off` setting: its OpenGL renderer writes its own messages onto the same pipe GDB is speaking on, and `gl off` in `blastem.cfg` selects the software renderer, which does not.

## MAME

MAME's stub listens on a TCP port instead of a pipe: this section describes MAME's Genesis driver as its own documentation gives it, not a session run here.

```sh
mame genesis -cart patched.md -debug -debugger gdbstub
```

`-debug` breaks after the initial soft reset; `-debugger gdbstub` opens the stub on port 23946 by default (`-debugger_port` changes it), on localhost (`-debugger_host`'s default) unless told otherwise. GDB connects with `target remote localhost:23946`. MAME's `m68000` register set uses the same names shown below, `d0`-`d7`, `a0`-`a5`, `fp`, `sp`, `ps`, `pc`, so `$a6` is `$fp` and the status register `$ps`, not `$sr`. Its stub implements `Z0` to `Z4`: software and hardware breakpoints and watchpoints both work. These come from MAME's documentation (docs.mamedev.org, "Debugging options") and `src/osd/modules/debugger/debuggdbstub.cpp`.

## A session

Commands typed at the `(gdb) ` prompt carry that prefix below; the lines under each are what the session printed back, paths shown relative to the project. GDB loads the symbol file as a plain argument, before any prompt:

```sh
gdb-multiarch build/m68k/patch.elf
```

```text
(gdb) set architecture m68k
The target architecture is set to "m68k".
(gdb) target remote | blastem patched.md -D
patch_entry () at hooks/hooks.s:15
15          jsr     patch_init
(gdb) break patch_init
Breakpoint 1 at 0xffff8: file src/patch.c, line 10.
(gdb) continue
Breakpoint 1, patch_init () at src/patch.c:10
10          volatile uint32_t a = 123456789u, b = 1000u, e = 3u;
(gdb) list
8       uint32_t patch_init(void)
9       {
10          volatile uint32_t a = 123456789u, b = 1000u, e = 3u;
11          volatile int32_t c = -100000, d = 7;
12          return (a / b) * e + (uint32_t)(c / d) + (a % b);
13      }
(gdb) next
11          volatile int32_t c = -100000, d = 7;
(gdb) next
12          return (a / b) * e + (uint32_t)(c / d) + (a % b);
(gdb) info locals
a = 123456789
b = 1000
e = 3
c = -100000
d = 7
(gdb) step
0x001000de in __udivsi3 (n=123456789, d=1000) at src/rt/div.c:24
24      {
(gdb) break patch_ret
Breakpoint 2 at 0x1001ce: file hooks/hooks.s, line 18.
(gdb) continue
Breakpoint 2, patch_ret () at hooks/hooks.s:18
18          jmp     0x200               | the original entry, as in .orig_000004
(gdb) p/d $d0
$1 = 356872
(gdb) x/2i $pc
=> 0x1001ce <patch_ret>:    jmp 0x200
   0x1001d2:    .short 0xffff
(gdb) x/4xw 0xFF0000
0xff0000:    0x00000000    0x00000000    0x00000000    0x00000000
(gdb) break *&wotes_reset_entry
Breakpoint 3 at 0x200
(gdb) continue
Breakpoint 3, 0x00000200 in ?? ()
(gdb) p &wotes_reset_entry
$2 = (<variable (not text or data), no debug info> *) 0x200
(gdb) info registers
d0             0x57208             356872
fp             0x0                 0x0
sp             0xfffffff6          0xfffffff6
ps             0x2700              [ I0 I1 I2 S ]
pc             0x200               0x200
(gdb) x/10i $pc
=> 0x200:    tstl 0xa10008
   0x206:    bnes 0x20e
   0x208:    tstw 0xa1000c
   0x20e:    bnes 0x28c
   0x210:    lea %pc@(0x28e),%a5
   0x214:    movemw %a5@+,%d5-%d7
   0x218:    moveml %a5@+,%a0-%a4
   0x21c:    moveb %a1@(-4351),%d0
   0x220:    andib #15,%d0
   0x224:    beqs 0x22e
(gdb) x/16xb 0xFF0000
0xff0000:    0x00    0x00    0x00    0x00    0x00    0x00    0x00    0x00
0xff0008:    0x00    0x00    0x00    0x00    0x00    0x00    0x00    0x00
```

`$d0` is 356872 once `patch_ret` is reached, matching the comment in `src/patch.c`. `step` went into `__udivsi3` since the division calls it; `next` would have stepped over it. `x/4xw` and `x/16xb` at `0xFF0000` both read zero: nothing here touches work RAM.

`break *&wotes_reset_entry` breaks on a symbol from `game/wotes.ld` with no C declaration: GDB has no type for the bare name (`p wotes_reset_entry` alone fails, "unknown type"), but its address works in `break` and `p` taken with `&`. It lands in the game's own code, reported as `?? ()` since nothing here has debug information; `x/10i $pc` matches [The ROM image](rom-image.md)'s account of this ROM's first instructions. `ps` shows supervisor mode with every interrupt masked, the state reset leaves it in.

## Watchpoints

```text
(gdb) watch *(unsigned short *)0xFF1234
Hardware watchpoint 1: *(unsigned short *)0xFF1234
```

BlastEm's stub answers as a hardware watchpoint: the target itself notices the write and stops, rather than GDB stepping one instruction at a time. A stub without one still accepts `watch` once told not to expect hardware support:

```text
(gdb) set can-use-hw-watchpoints 0
(gdb) watch *(unsigned short *)0xFF1234
Watchpoint 1: *(unsigned short *)0xFF1234
```

GDB then single-steps to catch the write, slower but independent of the stub. [Game symbols](game-symbols.md) uses the same `watch` to catch what writes to a RAM address already suspected; that page finds free RAM with a fill-and-dump instead, and confirms a routine with a breakpoint.

## A scripted check

The same connect-and-check sequence runs unattended, comparing `patch_init`'s result against a known value instead of a person reading the prompt.

```sh
#!/bin/sh
# Boots the patched ROM under BlastEm's GDB stub and checks what patch_init returns.
# Usage: check-target.sh ELF ROM EXPECTED
set -u
elf=$1; rom=$2; want=$3
out=$(timeout 120 gdb-multiarch -batch \
  -ex 'set architecture m68k' \
  -ex "target remote | blastem \"$rom\" -D" \
  -ex 'break patch_init' -ex 'continue' \
  -ex 'break patch_ret' -ex 'continue' \
  -ex 'p/d $d0' -ex 'kill' "$elf" 2>&1)
echo "$out" | grep -q "^\$1 = $want\$" || { echo "check-target: expected d0 = $want" >&2; echo "$out" >&2; exit 1; }
echo "check-target: patch_init returned $want"
```

Run once against a build with `blastem` on the PATH:

```text
check-target: patch_init returned 356872
```

`timeout` bounds the whole thing, so a wrong breakpoint or a build that never reaches `patch_ret` fails instead of hanging it. The script exits 0 on a match, as this run did, and 1 when `$d0` does not equal `want`.

## Reading the artifacts

`patch.map` shows where the linker placed every symbol; `readelf -S` and `objdump -d` read the same facts from `patch.elf`; `mdrom info patched.md` reads the ROM's own header instead. [The build pipeline](build-pipeline.md) covers all four with real output, unchanged by a session: injection moves no address `-g` already recorded.

## When something goes wrong

### The breakpoint never hits

Names above resolve because `-g` put them in the ELF; a bare name with no `.h` declaration, used alone rather than with `&`, fails to resolve rather than staying pending: `break *&wotes_reset_entry`, above, shows the same kind of symbol working once its address is taken instead. Check the spelling GDB reports back, and, at a hook site, that the build placed the hook where its `HOOKS` entry claims ([Hooks](hooks.md)).

### Address error

A word or long-word access at an odd address, or a stack pointer left odd, traps at vector `0x00C` ([The ROM image](rom-image.md); [Writing C](writing-c.md) shows the packed-pointer case). `info registers pc` after the trap points into the exception handler, not the faulting instruction; `x/i` on the address the handler saved gets back to it.

### Illegal instruction

Vector `0x010` fires for an opcode the 68000 lacks: most often a 68020 mnemonic emitted without `-m68000`, or a trampoline re-executing a displaced instruction across the wrong boundary, the tail of one decoded as the head of another ([Hooks](hooks.md), "Displaced instructions"). `x/i $pc` at the trap shows what decoded there.

### The game hangs after the hook

A hook reached with `jsr` that never returns, or a `jmp` redirected past the original flow, leaves the game spinning with nothing to report. A breakpoint on the hook confirms it runs; `stepi` follows the return or jump to see where control goes ([Hooks](hooks.md), "Returning").

### The checksum screen

A game that verifies the header's checksum at boot stops or warns when the sum it computes disagrees with the one stored there ([The ROM image](rom-image.md)). `mdrom inject` recomputes and writes that field, so a checksum failure on an injected ROM usually means the image was edited after `inject` ran, not that the patch is wrong; `mdrom info` shows both values.
