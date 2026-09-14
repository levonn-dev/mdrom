# Call a game routine

## Goal

Call an existing game routine from new patch code, through a bound symbol and, when its calling convention needs one, an assembly wrapper.

## What you need first

Finding a routine's address and confirming its register interface with a debugger, then binding it in `game/<name>.ld` and declaring it in `game/<name>.h`, is [Game symbols](../game-symbols.md). Moving C's stack arguments into the registers a routine expects, and back out again, is [Writing assembly](../writing-asm.md), "Wrapping a game routine"; calling a bound symbol directly from C, for a routine that takes no register arguments at all, is [Writing C](../writing-c.md), "Game symbols".

## The wrapper

`draw_text` wraps a routine bound to `game_draw_text`, which the game expects with its string pointer in `a0`, its position in `d0`, and which clobbers `d2` and `a2` along the way:

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

This is the same wrapper [Writing assembly](../writing-asm.md), "Wrapping a game routine", walks through in full: why the two arguments end up at `12(%sp)` and `18(%sp)` once the `movem.l` has pushed its eight bytes, and why `pos`, a `uint16_t`, comes from offset 2 of its 4-byte slot rather than offset 0.

## Binding the symbol

`game/<name>.ld` binds the address the wrapper's `jsr` reaches:

```ld
game_draw_text = 0xA646;   /* sample address, not a confirmed one in Warriors of the Eternal Sun */
```

`0xA646` is the same sample address used above; find the real one from a disassembly and confirm it with a breakpoint before binding it ([Game symbols](../game-symbols.md)).

## The C

```c
#include <stdint.h>

/* game/<name>.h declares the wrapper's C signature; the wrapper in a .s file moves the
 * arguments into the registers the game's routine expects. */
void draw_text(const char *s, uint16_t pos);

void greet(void)
{
    draw_text("HELLO", 42);
}
```

`greet` calls `draw_text` like any other C function taking a pointer and an integer; nothing about the game's own register convention reaches past the wrapper.

## Build and see it

```sh
cmake --preset m68k
cmake --build --preset m68k
```

Connect GDB as [Debugging](../debugging.md) sets up, break on `game_draw_text`'s bound address, and continue: the breakpoint fires with `a0` holding the string and `d0` the position `greet` passed in, exactly as the game's own callers leave them.
