# Hook mid-routine

## Goal

Run C from the middle of a game routine with a trampoline, then resume the routine exactly where the hook interrupted it.

## What you need first

Choosing a mid-routine site, why its bytes must be whole instructions, and the register and flag rules a trampoline follows are in [Hooks](../hooks.md), "Mid-routine" and "Displaced instructions". Finding the site itself is [Game symbols](../game-symbols.md); the trampoline is plain [Writing assembly](../writing-asm.md), the hooked function [Writing C](../writing-c.md).

## Choosing the site

The six bytes a mid-routine hook replaces must be whole instructions: cutting one in half leaves its remaining bytes decoded as something else. The disassembly around the site must also show nothing branches into the middle of that range, since anything that does would land inside the trampoline's copy, not the original. The offset `0x0123AC` and the two instructions it holds below are a sample site, not an address in Warriors of the Eternal Sun; find the real one from a disassembly and confirm it with a breakpoint before applying this recipe.

## The hook

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

`movem.l %d0-%d1/%a0-%a1,-(%sp)` saves the four registers C is free to clobber, and `move.w %sr,-(%sp)` saves the flags alongside them, since the 68000 has no direct move from the condition code register on its own ([Hooks](../hooks.md), "Registers and flags"). `jsr on_hook` runs the C side with everything the routine still needs preserved on the stack. The pops restore both, in reverse order, before the two displaced instructions run again exactly as the routine originally had them, and `rts` returns to `0x0123B2`, the byte after the six replaced.

Add `.hook_0123AC`/`.orig_0123AC` to `hooks/hooks.s` and append the site's real offset to `CMakeLists.txt`'s `HOOKS` list.

## The C

```c
/* Called from the trampoline with every register saved; it may do anything a
 * freestanding C function can, and must return. */
void on_hook(void)
{
}
```

Nothing about `on_hook`'s own body needs to know it runs mid-routine: the trampoline around it is what makes that safe, saving what the routine needs and putting the displaced instructions back before returning.

## Build and see it

```sh
cmake --preset m68k
cmake --build --preset m68k
```

Connect GDB as [Debugging](../debugging.md) sets up, break on `on_hook`, and continue: the breakpoint fires from inside the routine's normal flow, and stepping past it lands back on the two displaced instructions before the routine continues.
