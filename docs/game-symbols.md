# Game symbols

A project's `game/<name>.h` and `game/<name>.ld`, generated empty by `mdrom init`, hold every address a patch borrows from the game it patches: a routine to call, a byte of RAM to read, a table to point at ([Project layout](project-layout.md)). This page covers what each file holds, how to find an address by reading the ROM and by running it under GDB, how to find work RAM the game does not already use, and how to write an address down so it can be trusted later.

## The two files

`game/<name>.ld` binds a name to an address, one linker symbol per line and nothing else: `name = 0xADDRESS;`. `game/<name>.h` gives that name a C type: `extern` for a variable, a prototype for a routine. Both start empty:

```ld
/* Addresses of original game routines and RAM as linker symbols, e.g. game_draw_text = 0xA646;
   empty until the game's routines and work RAM have been mapped. */
```

```c nocheck
/* Prototypes for the original game routines named in wotes.ld; empty until they are mapped. */
#ifndef WOTES_H
#define WOTES_H
#endif
```

Every symbol carries a short prefix naming the project, `game_` in the generated file's own example above, so it cannot collide with a name mdrom's own runtime defines or a symbol from another game in the same tree. Any short prefix works; this page uses `wotes_`, the prefix for the Warriors of the Eternal Sun project used throughout this guide. Each entry also carries a comment saying how its address was found: a grep offset, a breakpoint that hit, a watchpoint that fired. 

## Finding routines and data

`objdump` reads the ROM as flat bytes and disassembles from any offset given, and starting at the reset vector target reads the game's own boot code the way the CPU does at power-on. The examples below are the Warriors of the Eternal Sun ROM used throughout this guide (SHA-1 `9135f7fda03ef7da92dfade9c0df75808214f693`):

```sh
ROM="Dungeons & Dragons - Warriors of the Eternal Sun (USA, Europe).md"
m68k-linux-gnu-objdump -D -b binary -m m68k --start-address=0x200 --stop-address=0x240 "$ROM"
```

```text
Disassembly of section .data:

00000200 <.data+0x200>:
     200:	4ab9 00a1 0008 	tstl 0xa10008
     206:	6606           	bnes 0x20e
     208:	4a79 00a1 000c 	tstw 0xa1000c
```

The first instruction is `tstl 0xa10008` (objdump drops the dot before the size suffix), a read from the I/O range, not cartridge space ([The ROM image](rom-image.md)). Every fixed offset in the vector table is a 4-byte address, not an instruction ([The ROM image](rom-image.md)), so reading one gives an address to disassemble or break on without running anything. The vertical-blank vector, offset 0x78, is one the Mega Drive dispatches to every frame:

```sh
xxd -s 0x78 -l 4 "$ROM"
```

```text
00000078: 0000 09e6                                ....
```

`0x000009E6` is this ROM's vertical-blank handler: the routine to follow into the frame loop.

A string in the game's own text, an item name, a debug label GCC left behind, exists in ROM as the bytes it prints, so a plain byte search finds every place it sits:

```sh
grep -abo FIGHTER "$ROM"
```

```text
8929:FIGHTER
8982:FIGHTER
892431:FIGHTER
```

Two offsets close together suggest the same table; the third, far off, is worth treating as a different one until proven otherwise.

A pointer to a known address, rather than text at it, is a fixed 4-byte big-endian value. This ROM's reset vector target, `0x00000200`, is the bytes `00 00 02 00`:

```sh
printf '\x00\x00\x02\x00' | xxd
```

```text
00000000: 0000 0200                                ....
```

A shell string cannot hold an embedded NUL byte, so `$'\x00\x00\x02\x00'` does not survive as an argument; the search needs a tool that interprets the escapes itself instead of relying on the shell to expand them first. `grep -P` does, reading `\x00\x00\x02\x00` as a Perl-compatible pattern and matching the ROM's raw bytes directly.

```sh
grep -abo -P '\x00\x00\x02\x00' "$ROM" | head -3
```

The three offsets it prints are 4, 8, and 12 (the matched bytes are control characters and do not render in a terminal). Offset 4 is the reset vector itself; 8 and 12 are the bus-error and address-error vectors, 0x008 and 0x00C in the table ([The ROM image](rom-image.md)), which in this ROM also hold `0x00000200` rather than a dedicated handler.

Static analysis proposes a routine; a GDB session, connected as [Debugging](debugging.md) sets up, confirms it runs when expected. Breaking on the vertical-blank handler found above stops execution there, whether the CPU reaches it by a call or by the hardware interrupt that fires each frame:

```text
(gdb) break *0x9e6
(gdb) continue
(gdb) info registers pc
```

A watchpoint catches a write to a candidate RAM address before anything is known about what reads or writes it. `0xFF0100` here is a sample address, not a confirmed one, the same sample used below in Recording what you found:

```text
(gdb) watch *(unsigned char *)0xff0100
(gdb) continue
```

GDB stops the instant that byte changes and names the instruction that changed it. Stepping from the reset vector walks the CPU's own path through boot one instruction at a time instead of guessing where a routine starts:

```text
(gdb) break *0x200
(gdb) continue
(gdb) display/i $pc
(gdb) stepi
(gdb) stepi
```

Each `stepi` prints the next instruction; a `jsr` or `bsr` in the trace is a call into a routine worth naming.

## Finding free work RAM

mdrom's static tools show only what is in the ROM's bytes; which of the console's work RAM a running game actually leaves alone is not one of them, since that is a fact about behavior, not content. The header's `ram` field names the console's whole work-RAM window, `0xFF0000-0xFFFFFF` for this ROM (`mdrom info`, [The ROM image](rom-image.md)), not the part of it this game happens to use. Finding a genuinely free range takes running the game and watching what it touches.

Fill a candidate range with a pattern the game is unlikely to write on its own, after its own boot-time clear has already run so the fill is not immediately overwritten by code expecting zeroed RAM. A `while` loop needs no separate pattern file, unlike a `restore` of a prepared binary, so the whole fill is commands typed into the same GDB session:

```text
(gdb) set $p = 0xff8000
(gdb) while $p < 0xff9000
(gdb)   set {unsigned char}$p = 0xa5
(gdb)   set $p = $p + 1
(gdb) end
(gdb) continue
```

Let the game run for a while, played rather than merely resumed, then interrupt it (Ctrl-C at the GDB prompt) and dump the same range:

```sh
dump binary memory fill.bin 0xff8000 0xff9000
```

Compared against the fill pattern outside GDB, the bytes still `0xa5` are the ones nothing wrote during that session: candidates for free RAM. A range the game's own init clears is worth preferring, since a wrong guess there is wiped again on the next boot instead of leaving a stale pattern behind.

The stack's low-water mark uses the same fill, play, and dump, over a range below where the stack pointer starts:

```text
(gdb) set $bottom = $sp - 0x1000
(gdb) set $p = $bottom
(gdb) while $p < $sp
(gdb)   set {unsigned char}$p = 0xa5
(gdb)   set $p = $p + 1
(gdb) end
(gdb) continue
(gdb) dump binary memory stack.bin $bottom $sp
```

The lowest address in that dump no longer `0xa5` is the deepest the stack reached during play; everything below it, down to the bottom of the filled range, went untouched and is as free as any other candidate range, once played long enough to trust the result. [Add RAM variables](recipes/add-ram-variables.md) covers placing a new global in a range found this way.

## Recording what you found

An address is only as good as the session that found it. The `.ld` line binds it, the `.h` declaration gives it a type, and a comment on each records the method, not just the result, so a later rebuild or a different reader can tell a breakpoint that fired from a guess that happened to compile:

```c
#include <stdint.h>

/* game/wotes.h: one declaration per symbol in game/wotes.ld, typed for C.
 * The addresses in the comments are samples; each real one records how it was found. */
extern volatile uint8_t wotes_party_gold;   /* wotes_party_gold = 0xFF0100; watched in GDB while buying */
void wotes_show_menu(void);                 /* wotes_show_menu = 0xA700; broken on from the reset vector */

uint8_t gold(void)
{
    return wotes_party_gold;
}
```

`0xFF0100` and `0xA700` are sample addresses for this example, not facts about Warriors of the Eternal Sun; the matching `.ld` entries carry the same two comments:

```ld
wotes_party_gold = 0xFF0100;   /* watched in GDB while buying */
wotes_show_menu  = 0xA700;     /* broken on from the reset vector */
```

`wotes_show_menu` takes no register arguments in this example, so C calls it directly; a routine the game calls through specific registers instead needs an assembly wrapper first ([Writing C](writing-c.md), [Writing assembly](writing-asm.md)). The debugger session that found an address, not the compiled snippet, is what makes it trustworthy: a watchpoint that fired while gold changed, or a breakpoint that hit on the way to the menu, is what the comment is short for, and rerunning it after a rebuild is how a stale address gets caught before it reaches a patch.
