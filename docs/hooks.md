# Hooks

A hook is the place in a Mega Drive ROM a patch built with mdrom changes bytes the game reads or executes: a fixed offset overwritten with something leading to new code, checked against the clean ROM ([How a patch works](how-a-patch-works.md)). This page covers the convention, the reset-vector hook, seven kinds of site, instruction sizes, displaced instructions, registers and flags, returning, and what the checks miss.

## The convention

A hook section is named `.hook_XXXXXX`, six hex digits for its offset, and is allocatable: `"ax"` for code, `"a"` for data. mdrom rejects one that is not allocatable, that is empty, that is bss, or that is linked at the wrong address (`internal/elfimg/elfimg.go`). Its `.orig_XXXXXX` twin, non-allocatable, holds the clean-ROM bytes expected, checked against the base ROM before any write.

`CMakeLists.txt`'s `HOOKS` list names every offset and generates its `--section-start` flag, a hook's only address source. mdrom refuses a hook without a same-size `.orig_` twin, and an `.orig_` without its hook; `check-hooks.sh` repeats that pairing check at link time and adds what mdrom cannot see: every `HOOKS` entry must be linked, and every linked `.hook_` section must appear in `HOOKS`.

## The reset-vector hook

`hooks/hooks.s`, rendered by `mdrom init` for the Warriors of the Eternal Sun ROM (entry `0x200`):

```asm
| Each .hook_XXXXXX section is placed at ROM offset XXXXXX by the HOOKS list in
| CMakeLists.txt; its .orig_ twin holds the clean-ROM bytes mdrom checks before writing.

| Reset vector. The console loads the stack pointer from offset 0 and the program
| counter from offset 4, so patch_entry runs before the game with a valid stack and
| every other register undefined. TMSS consoles take the same vector after their check.
    .section .hook_000004,"a"
    .long   patch_entry
    .section .orig_000004,""
    .long   0x00000200

    .text
    .globl  patch_entry
patch_entry:
    jsr     patch_init
    .globl  patch_ret
patch_ret:
    jmp     0x200               | the original entry, as in .orig_000004

    .section .note.GNU-stack,"",%progbits
```

`.hook_000004` holds a pointer to `patch_entry`; `.orig_000004` records the clean ROM's entry, `0x00000200`. `jsr patch_init` runs the self-test; `patch_ret` then jumps to `0x200`.

`jsr` is safe here only because reset leaves nothing to protect: the stack was set, no register holds a live value. Every other site saves what it disturbs ("Registers and flags").

## Choosing a site

### The reset vector

Safe: nothing is live yet, no register, stack, or in-flight call to preserve, only the vector's four bytes. Check: `.orig_000004` records the entry address for mdrom's comparison.

### Padding inside code

Safe when the bytes are provably never executed or read, such as alignment filler placed after an `rts`, once confirmed that nothing branches or falls into the range ("Displaced instructions"). Check: the run is long enough for the encoding ("Instruction sizes") and stays unread on every path into it.

### Redirecting a jsr or jmp

The replaced call is one instruction of known size; its register and stack state belong to the caller. Six bytes at `0x00456A` (a sample site; use the one you found) held one jsr to the game's text routine:

```asm
| The game calls its text routine with "jsr (0xA646).l" at 0x00456A (sample); the hook
| sends that call through count_call first. game_draw_text is bound in game/<name>.ld.
    .section .hook_00456A,"ax"
    jsr     draw_text_hook
    .section .orig_00456A,""
    .short  0x4EB9                  | jsr (xxx).l
    .long   0x0000A646

    .text
    .globl  draw_text_hook
draw_text_hook:
    movem.l %d0-%d1/%a0-%a1,-(%sp)  | the routine's register arguments survive the C call
    jsr     count_call
    movem.l (%sp)+,%d0-%d1/%a0-%a1
    jmp     game_draw_text          | the original, with the caller's return address still on the stack
    .section .note.GNU-stack,"",%progbits
```

`jmp game_draw_text` reaches the original; the caller's return address is on the stack from its `jsr`. Check: `.orig_00456A` matches the encoding, and every register the target expects holds it.

### Jump tables and the vector table

Safe like the reset vector: an entry is a pointer, not instructions, so replacing one only changes where flow goes. The exception vector table ([The ROM image](rom-image.md)) is one table of distinct entries, levels 2, 4, and 6 each at their own offset; a game may point several of them at the same handler, so hooking the handler's code, rather than one vector, changes every vector that names it. Check: `.orig_` against the current pointer; that a computed index cannot select a different entry elsewhere; and, before hooking a handler's code rather than its vector, which other vectors also name it.

### Mid-routine

Safe when the site's bytes are whole instructions a trampoline can save, replace, and re-execute ("Displaced instructions"), and the registers and flags needed after survive the extra call ("Registers and flags"). Six bytes at `0x0123AC` (a sample site; use the one you found) held two instructions:

```asm
| Six bytes at 0x0123AC (a sample site; use the one you found) held two instructions:
| move.w %d1,%d0 (3001) and lea 8(%a0),%a0 (41E8 0008). The jsr replaces both, and
| the trampoline re-executes them before returning to 0x0123B2.
    .section .hook_0123AC,"ax"
    jsr     hook_0123AC
    .section .orig_0123AC,""
    .short  0x3001                  | move.w %d1,%d0
    .short  0x41E8,0x0008           | lea 8(%a0),%a0

    .text
    .globl  hook_0123AC
hook_0123AC:
    movem.l %d0-%d1/%a0-%a1,-(%sp)  | C clobbers these four
    move.w  %sr,-(%sp)              | and the flags; the 68000 cannot save %ccr on its own
    jsr     on_hook
    move.w  (%sp)+,%ccr
    movem.l (%sp)+,%d0-%d1/%a0-%a1
    move.w  %d1,%d0                 | the displaced instructions
    lea     8(%a0),%a0
    rts
    .section .note.GNU-stack,"",%progbits
```

Check: the displaced instructions are whole and nothing branches into the range; every register and flag read afterward is restored exactly; the trampoline returns after the displaced bytes, `0x0123B2`.

### Data

A data hook never touches flow: it changes only bytes a routine loads or displays, declared `"a"`, not `"ax"`. `grep -abo FIGHTER` against the clean ROM prints `8929` (`0x0022E1`), `8982`, and `892431`; the first is what this hook replaces:

```asm
| Class name on the character-creation screen of Warriors of the Eternal Sun.
    .section .hook_0022E1,"a"
    .ascii  "WARRIOR"
    .section .orig_0022E1,""
    .ascii  "FIGHTER"
```

`WARRIOR` and `FIGHTER` are both seven bytes, so nothing shifts. Check: `.orig_` against the exact bytes, and whether the string is read elsewhere; the grep above shows two more `FIGHTER` offsets a different hook would need.

### Replacing a routine

Safe when a whole subroutine is replaced and every caller-visible detail stays the same: the argument and result registers, and how it is reached, entered, and left, whether with `jsr` and `rts` or with `jmp` and `jmp`. Every instruction inside can be new; nothing a caller depends on changes. Check: every call site's expectations, gathered first and confirmed afterward with a debugger.

## Instruction sizes

An instruction-size experiment gives the encoding and size a site needs:

| Instruction | Encoding | Bytes |
|---|---|---|
| `nop` | `4E71` | 2 |
| `bra.s` | `60xx` | 2 |
| `bra.w` | `6000 xxxx` | 4 |
| `bsr.w` | `6100 xxxx` | 4 |
| `jmp` absolute short | `4EF8 0200` | 4 |
| `jmp` absolute long | `4EF9 0010 0200` | 6 |
| `jsr` absolute short | `4EB8 7FFE` | 4 |
| `jsr` absolute long | `4EB9 0010 0200` | 6 |

The M68000 Family Programmer's Reference Manual gives each range: `bra.s` reaches -128 to +127 from the instruction's address plus 2; `bra.w`/`bsr.w` reach -32768 to +32767; absolute short reaches `0x0000`-`0x7FFF` and, sign-extended, `0xFFFF8000`-`0xFFFFFFFF`.

Patch code in `ROM_FREE` sits outside both short ranges for most sites, so reaching it usually needs six bytes, an absolute `jmp` or `jsr`; a site within 32KB of `ROM_FREE` needs only four, a `bra.w` or `bsr.w` reaching it directly. A site with only four bytes to spare and out of that range needs a trampoline within reach instead, one that carries the six-byte jump the rest of the way.

## Displaced instructions

A displaced instruction must move whole: landing mid-instruction leaves the remaining bytes decoded as a new one. `.orig_0123AC` records two complete instructions, re-executed unchanged before the trampoline returns.

Nothing in the ROM may branch into a displaced range: code reaching the second instruction would land in the trampoline's copy, not the original.

A PC-relative instruction cannot move as is: its displacement, run from the trampoline's address, reaches somewhere else, so it needs rewriting with absolute addressing.

## Registers and flags

C on this toolchain clobbers `d0`, `d1`, `a0`, and `a1`; a hook calling into C must save and restore them, which `movem.l %d0-%d1/%a0-%a1,-(%sp)` and its pop do in one instruction each.

The 68000 has no `move` from the condition code register on its own: `move.w %ccr,-(%sp)` fails with `operands mismatch`, which is why the whole status register has to be saved instead. `move.w %sr,-(%sp)` saves it entirely: the condition codes in the low byte, and, in the system byte, the supervisor bit (S), the trace bit (T), and the three-bit interrupt mask (I2-I0), per the M68000 Family Programmer's Reference Manual. `move.w (%sp)+,%ccr` restores only the low byte, the condition codes. A Mega Drive game runs in supervisor mode, so reading `%sr` is never privileged.

A hook adds to whatever stack depth is in use, each saved register or `jsr` a push, so one deep in a call chain or interrupt handler must not exceed the stack's room. The usable depth at any given site is unknown, so C called from a hook keeps its locals small.

A hook runs with whatever interrupt mask the game already had at that site; nothing raises it on entry. The VDP's address is set by two consecutive word writes to the control port (Sega's Mega Drive/Genesis software development manual), so if an interrupt handler that touches the VDP fires between them, the pending address is corrupted. The game's own routines avoid this by masking interrupts around such a sequence (`move.w #0x2700,%sr`, restored afterward) or by doing it inside the vertical-blank handler; a hook writing to the VDP must do the same.

## Returning

A hook reached through `jsr` returns with `rts`. One reached by redirecting a `jmp` jumps on to wherever that flow went, since no return address was pushed; "Redirecting a jsr or jmp" does this with `jmp game_draw_text`, leaving the caller's return address on the stack as the original `jsr` would.

A displaced instruction runs first only for a mid-routine hook, as above; the reset-vector hook has none to run, since it replaces a pointer, not an instruction, and jumps straight to the original entry. Either way, whatever runs next sees the state it would without the hook.

## What the checks cannot catch

mdrom's `.orig_` check and `check-hooks.sh`'s pairing check compare bytes: an offset holding the expected bytes by coincidence, not because it is the routine meant, passes both and writes anyway. Nothing at build time confirms a hook is reached, or reached when expected, once per frame, at boot, or from one menu; that takes a debugger and a breakpoint firing as expected. [Debugging](debugging.md) covers connecting one and reading a session against a patch.
