# Run code at boot

## Goal

Run C code before the game's own boot code, from the generated reset-vector hook, and know when a later site serves better.

## What you need first

Reset and TMSS are covered in [The ROM image](../rom-image.md); the generated hook that already occupies the reset vector is in [Hooks](../hooks.md). Growing what a hook calls, or moving it, changes the `HOOKS` list in `CMakeLists.txt` ([Project layout](../project-layout.md)).

## What is not set up yet

`mdrom init` writes a reset-vector hook that calls `patch_init` before jumping to the game's own entry point. At that point nothing is initialized: no VDP setup, RAM as the console left it, interrupts masked by the reset itself ([The ROM image](../rom-image.md)). On a console with the TMSS boot check, the VDP ignores writes until `SEGA` is written to `0xA14000`, and the I/O version register at `0xA10001` has non-zero low four bits on such a console. Code that only computes, like the generated self-test, needs none of this; code that touches the VDP does, as the `patch_init` under "The C" below shows.

## Moving to a later site

A patch that needs the game's own setup already done, a stack pointer in place, RAM cleared, waits for a site further into boot instead of the reset vector. The Warriors of the Eternal Sun ROM has three `nop`s at `0x300`, right before the game sets its own stack pointer:

```sh
m68k-linux-gnu-objdump -D -b binary -m m68k --start-address=0x300 --stop-address=0x310 "Dungeons & Dragons - Warriors of the Eternal Sun (USA, Europe).md"
```

```text
Disassembly of section .data:

00000300 <.data+0x300>:
     300:	4e71           	nop
     302:	4e71           	nop
     304:	4e71           	nop
     306:	4ff9 ffff fff6 	lea 0xfffffff6,%sp
     30c:	46fc 2700      	movew #9984,%sr
```

The three `nop`s do nothing, so replacing them re-executes nothing. `lea 0xfffffff6,%sp` at `0x306` is the game setting up the same stack pointer the reset vector already loaded ([The ROM image](../rom-image.md)), so a hook here still has a valid stack without setting one up itself.

## The hook

```asm
| The three nops at 0x300 in Warriors of the Eternal Sun, a site inside the game's own
| startup code. patch_entry sets the stack pointer the game sets at 0x306 and resumes there.
    .section .hook_000300,"ax"
    jmp     patch_entry
    .section .orig_000300,""
    .byte   0x4e,0x71,0x4e,0x71,0x4e,0x71

    .text
    .globl  patch_entry
patch_entry:
    lea     0xfffffff6,%sp
    jsr     patch_init
    .globl  patch_ret
patch_ret:
    jmp     0x306
    .section .note.GNU-stack,"",%progbits
```

`HOOKS` becomes `000300`; the generated `000004` pair is deleted, since the reset vector no longer holds a hook.

## The C

`patch_init` makes the TMSS write before anything touches the VDP; the same function serves at the reset vector and at the `0x300` site:

```c
#include <stdint.h>

#define VERSION_REG (*(volatile uint8_t *)0xA10001)
#define TMSS_REG    (*(volatile uint32_t *)0xA14000)

/* Runs before the game's own setup, from the reset vector or the 0x300 site. A patch that
 * touches the VDP here must unlock it on TMSS consoles first, as the game will
 * do again later; a patch that only computes, like the generated self-test, need not. */
void patch_init(void)
{
    if (VERSION_REG & 0x0F) {
        TMSS_REG = 0x53454741u;   /* "SEGA" */
    }
}
```

## Build and see it

```sh
cmake --preset m68k
cmake --build --preset m68k
```

Connect GDB as [Debugging](../debugging.md) sets up, break on `patch_init`, and continue: the breakpoint hits after the game's own three `nop`s and stack setup have run, not before them.
