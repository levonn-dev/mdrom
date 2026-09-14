# Replace a routine

## Goal

Replace a whole game routine with new code written in C, keeping the register interface every caller already expects.

## What you need first

Which sites tolerate replacing a whole routine, and what stays fixed across the boundary, arguments, results, how it is entered and left, are covered in [Hooks](../hooks.md), "Replacing a routine". Finding the routine's address and its calling convention takes reading the disassembly and confirming it with a debugger ([Game symbols](../game-symbols.md)). The shim below is plain [Writing assembly](../writing-asm.md); the replacement itself is [Writing C](../writing-c.md).

## Before hooking

The routine's first six bytes must be whole instructions, not a fragment of one, and the disassembly must show nothing else in the ROM branches into the middle of that range: a caller that jumps in halfway lands on whatever the hook leaves behind. Every register the routine's callers pass in and read back out has to be gathered first, since the C replacement takes over the routine's whole calling convention, not just the instructions it replaces.

The offset `0x00A646` and the two instructions below are a sample site and sample bytes, not a routine address in Warriors of the Eternal Sun; find the real ones from a disassembly and confirm them with a breakpoint before applying this recipe.

## The hook

```asm
| Replaces the game's routine at 0x00A646 (sample). The game enters it with the string in
| a0 and the position in d0 and expects nothing back. The shim passes both to C on the
| stack and returns to the game's caller with rts, so the C function is the whole routine.
    .section .hook_00A646,"ax"
    jmp     draw_text_replacement
    .section .orig_00A646,""
    .short  0x48E7,0xFFFE           | movem.l %d0-%d7/%a0-%a6,-(%sp)
    .short  0x2048                  | movea.l %a0,%a0 (sample bytes: use the real ones)

    .text
    .globl  draw_text_replacement
draw_text_replacement:
    move.l  %d0,-(%sp)              | pos, promoted to a 4-byte slot
    move.l  %a0,-(%sp)              | s
    jsr     draw_text_in_c
    addq.l  #8,%sp
    rts
    .section .note.GNU-stack,"",%progbits
```

The shim promotes each register argument into the 4-byte stack slot C expects ([Writing C](../writing-c.md)), calls the replacement, and pops both slots back off before `rts`. Add `.hook_00A646`/`.orig_00A646` to `hooks/hooks.s` and append the routine's real offset to `CMakeLists.txt`'s `HOOKS` list.

## The C

```c
#include <stdint.h>

/* The replacement, called by the shim with the game's a0 and d0 as arguments. */
void draw_text_in_c(const char *s, uint32_t pos)
{
    (void)s;
    (void)pos;
}
```

Every argument the routine's callers relied on arrives as a normal C parameter; nothing about the replacement's internals has to match the routine it replaces, only its interface.

## Build and see it

```sh
cmake --preset m68k
cmake --build --preset m68k
```

Connect GDB as [Debugging](../debugging.md) sets up, break on `draw_text_in_c`, and continue: the breakpoint fires wherever the game used to call the original routine, with `s` and `pos` holding the values the game passed in `a0` and `d0`.
