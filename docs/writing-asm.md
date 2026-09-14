# Writing assembly

Some patch code cannot be C at all: a routine the game calls with arguments already in registers, not on the stack, or a hook that must fit in exactly as many bytes as the site it replaces ([Hooks](hooks.md)). mdrom assembles `.s` files with the same cross compiler used for C, `m68k-linux-gnu-gcc -m68000 -x assembler -c` ([The build pipeline](build-pipeline.md)), through GNU `as`'s syntax for the 68000, not Motorola's. This page covers that syntax, its directives, calling C and being called from it, wrapping a game routine with a register interface, data, and where the assembler stops helping.

## GNU as against Motorola syntax

A comment runs from `|` to end of line. A register is `%d0`-`%d7`, `%a0`-`%a7` (`%sp` for `%a7`), or `%pc`; an immediate is `#`-prefixed (`#3`). A size suffix, `.b`, `.w`, or `.l`, picks byte, word, or long; a branch takes `.s` for the one-byte form instead. Operand order is source, then destination, the same as Motorola's own syntax; the addressing-mode spellings are what differ. `(%a0)` is register indirect, `(%a0)+` postincrement, `-(%a0)` predecrement, `4(%a0)` a displacement, and `1b(%pc)` PC-relative.

An absolute address can be written three ways, and the assembler treats them as the same operand:

```sh
cat > syn.s <<'S'
    .text
    bra.s   1f
    nop
1:  jmp     0x200
    jmp     (0x200).w
    jmp     0x200:w
    jsr     0x100200
    lea     1b(%pc),%a0
S
m68k-linux-gnu-gcc -m68000 -c syn.s -o syn.o && m68k-linux-gnu-objdump -d syn.o
```

Assembled and disassembled, all three `jmp 0x200` lines produce identical code, and `jsr` to the larger address gets a wider encoding:

```text
   4:	4ef8 0200      	jmp 0x200
   8:	4ef8 0200      	jmp 0x200
   c:	4ef8 0200      	jmp 0x200
  10:	4eb9 0010 0200 	jsr 0x100200
  16:	41fa ffec      	lea %pc@(0x4),%a0
```

Bare `0x200`, `(0x200).w`, and `0x200:w` all pick absolute short (`4ef8 0200`, four bytes): the value fits `0x0000`-`0x7FFF`. `0x100200` does not, so `jsr` picks absolute long (`4eb9 0010 0200`, six bytes) unasked. A `.l` suffix forces long regardless of magnitude: `(0x200).l` assembles to `4ef9 0000 0200`, not the short form. The `lea` line is PC-relative to `1b`, the label one line back; objdump renders it in MIT style, `%pc@(0x4)`, the same style it gives stack offsets like `%sp@(8)` ([Writing C](writing-c.md)).

## Directives

`.section NAME,"FLAGS"` starts a section; `"ax"` marks one allocatable and executable, `"a"` allocatable data, and `""` neither, as the hook sections in [Hooks](hooks.md) show. `.text` switches to the code section; `.globl NAME` exports a symbol so other files, and the linker, can see it. `.byte`, `.short`, `.long`, `.ascii`, and `.asciz` (the last NUL-terminated) lay down data.

`.balign N` pads to an N-byte boundary and says so in its name; `.align N` does the same thing on this target, padding to N bytes rather than to `2^N`. Assembling a `.rodata` section with a byte, a `.balign 2`, another byte, an `.align 4`, and a third byte:

```text
 0000 01000200 03                          .....
```

places the second byte at offset 2 and the third at offset 4, the byte counts `.balign 2` and `.align 4` asked for, not the two- and four-bit shifts a power-of-two reading would imply. `.set NAME,VALUE` defines a symbol without emitting anything. A numeric label like `1:` can repeat through a file; `1f` refers to the next one forward, `1b` to the nearest one back. The `bra.s` and `lea` lines above reach their targets this way, without needing a unique name for each one.

The final line in every snippet, `.section .note.GNU-stack,"",%progbits`, is empty and allocates nothing; it states that this object needs no executable stack. The compiler adds that marker to a C object on its own; an assembly file gets one only by declaring it, which is why every `.s` file in this project, and every `asm` snippet in this guide, carries the line explicitly ([The build pipeline](build-pipeline.md)). A section claiming the opposite trips the linker: one flagged `"x"` for executable, linked with `-Wl,-z,noexecstack`, warns: `requires executable stack (because the .note.GNU-stack section is executable)`. Leaving the line out is not the same mistake: a `.s` file assembled without it still links, since `-z noexecstack` decides the final `GNU_STACK` permission either way, regardless of which objects carry the marker.

## Being called from C

C calls an assembly routine like any other function: arguments in 4-byte stack slots from `4(%sp)` upward, a result in `d0`, `d2`-`d7` and `a2`-`a6` left unchanged ([Writing C](writing-c.md) shows this, with disassembly). `swap_words` takes one `uint32_t` argument and returns it with its two 16-bit halves swapped:

```asm
| uint32_t swap_words(uint32_t v): the argument is at 4(%sp), above the return address;
| the result goes in d0; d2-d7 and a2-a6 must come back unchanged, so this uses neither.
    .text
    .globl  swap_words
swap_words:
    move.l  4(%sp),%d0
    swap    %d0
    rts
    .section .note.GNU-stack,"",%progbits
```

Nothing here needs saving: the routine uses only `d0`, which is fair game for a result anyway. One that does use `d2`-`d7` or `a2`-`a6` saves and restores them with `movem.l`, as the wrapper below shows.

## Calling C

Calling C from assembly reverses the direction: push the arguments right to left, `jsr`, then pop what was pushed, and expect `d0`, `d1`, `a0`, and `a1` to be gone. `call_add3` calls `add3(uint32_t a, uint32_t b, uint32_t c)`:

```asm
| Calls uint32_t add3(uint32_t a, uint32_t b, uint32_t c): push right to left, jsr,
| pop the twelve bytes of arguments. The result is in d0; d1, a0, and a1 are gone.
    .text
    .globl  call_add3
call_add3:
    move.l  #3,-(%sp)
    move.l  #2,-(%sp)
    move.l  #1,-(%sp)
    jsr     add3
    lea     12(%sp),%sp
    rts
    .section .note.GNU-stack,"",%progbits
```

`c` is pushed first so `a`, pushed last, ends up closest to the return address, matching the `4(%sp)`-upward layout `add3` expects. Popping the twelve bytes back needs `lea 12(%sp),%sp` here; `addq.l` only takes an immediate of 1 to 8, enough for one or two 4-byte arguments but not three.

## Wrapping a game routine

A game routine rarely uses the C convention: it expects arguments already in specific registers and may clobber registers of its own. Wrapping one moves C's stack arguments into those registers, saves what the routine disturbs, calls it, and, if it returns a value, moves that into `d0`. `draw_text` wraps a routine bound to `game_draw_text` in `game/<name>.ld` ([Writing C](writing-c.md) covers that binding), which wants its string pointer in `a0`, its position in `d0`, and clobbers `d2` and `a2`:

```asm
| void draw_text(const char *s, uint16_t pos) for C: the game's routine wants the string
| in a0 and the position in d0 and also clobbers d2 and a2. game_draw_text is bound in
| game/<name>.ld. After the movem, the first C argument sits at 12(%sp).
    .text
    .globl  draw_text
draw_text:
    movem.l %d2/%a2,-(%sp)
    move.l  12(%sp),%a0             | s
    move.w  18(%sp),%d0             | pos: the low word of its 4-byte slot
    jsr     game_draw_text
    movem.l (%sp)+,%d2/%a2
    rts
    .section .note.GNU-stack,"",%progbits
```

`movem.l %d2/%a2,-(%sp)` pushes eight bytes before either argument is read, so the two arguments that would sit at `4(%sp)` and `8(%sp)` on entry move to `12(%sp)` and `16(%sp)`. `s` is a pointer, read whole from `12(%sp)`. `pos` is a `uint16_t`, zero-extended into its 4-byte slot ([Writing C](writing-c.md)), so its value is the low word at offset 2 of that slot: `16 + 2 = 18`, the offset `move.w 18(%sp),%d0` reads. `jsr` calls the routine with both registers set; the `movem.l` pop after it restores what C owns before `rts`.

## Data

Data belongs in `.rodata`, aligned to whatever access reads it. `region_names` is a table of three NUL-terminated strings, word-aligned so a `move.w` reading it cannot land on an odd address:

```asm
| A table of NUL-terminated names, word-aligned so a move.w from it cannot fault.
    .section .rodata
    .balign 2
    .globl  region_names
region_names:
    .asciz  "VALLEY"
    .asciz  "JUNGLE"
    .asciz  "SWAMP"
    .section .note.GNU-stack,"",%progbits
```

`VALLEY`, `JUNGLE`, and `SWAMP` are outdoor region names in the Warriors of the Eternal Sun ROM, appearing together at offsets 40372, 40382, and 40392.

## Pitfalls

`-m68000` rejects a 68020-only mnemonic outright: `bsr.l` and `extb.l` both fail with `invalid instruction for this architecture`. A related mistake is an operand no 68000-family part ever accepted: `move.w %ccr,-(%sp)` fails with `operands mismatch`, since the 68000 has no direct move from the condition code register ([Hooks](hooks.md) covers saving flags through `%sr` instead). Both are compile-time errors.

Not caught at compile time is a prebuilt object linked in from elsewhere, never passed through `-m68000`. `check-68000.sh` disassembles the whole ELF after the link and rejects any 68020-only mnemonic or mode it finds, reasoning that "our code is built with -m68000, so a hit means a prebuilt object such as libgcc crept in." A per-file compile catches a typo; this scan catches an import.

Forgetting `.globl` compiles clean: a file assembling `swap_words` without exporting it passes `-c` with no complaint, since nothing there needs the symbol visible. The failure shows up only when another file tries to call it, at the final link: `` undefined reference to `swap_words' ``.

A `.w`-forced absolute address above `0x7FFF` does not necessarily error. `(0x8100).w` assembles silently to `4ef8 8100`, which disassembles as `jmp 0xffff8100`, the sign-extended address, not `0x8100`. Only once the value cannot fit sixteen bits at all does the assembler complain, and it still keeps going: `(0x10000).w` prints `expression doesn't fit in WORD` and encodes `0x0`.

`bra.s` to the very next instruction is a genuine error, not a silent wrong answer: a zero displacement is reserved to select the word form, so `bra.s` immediately followed by its target fails with `invalid byte branch offset`.
