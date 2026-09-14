# Writing C

Patch code compiles with `m68k-linux-gnu-gcc -m68000 -ffreestanding -nostdlib -fno-pic -fomit-frame-pointer -Os -Wall -Wextra -g` ([The build pipeline](build-pipeline.md)): a full C compiler for the 68000 with no operating system and no C library beneath it. This page covers what it still needs, how it passes and returns values, type sizes, hardware and game-memory access, game symbols, data placement, and generated code.

## What freestanding GCC provides

`-ffreestanding` tells GCC not to assume a hosted environment; `-nostdlib` drops the startup files and libraries at the link step ([The build pipeline](build-pipeline.md)). What remains is the C language itself, plus five headers this guide uses: `stdint.h`, `stddef.h`, `stdbool.h`, `limits.h`, and `stdarg.h`. C11 also lists `float.h`, `iso646.h`, `stdalign.h`, and `stdnoreturn.h` as freestanding, compiling here too. The sizes-and-alignment snippet below includes all five and compiles clean, proof they work here. Nothing else in libc exists: no `stdio.h`, no `malloc`, no `string.h` function, though the compiler can still call a few of those names.

## What the compiler still calls

Four names come from `src/rt/div.c` and one from `src/rt/mul.s`: `__udivsi3`, `__umodsi3`, `__divsi3`, `__modsi3`, and `__mulsi3`, the 32-bit arithmetic helpers the 68000's 16x16 multiply and missing divide instruction need once `-m68000` rules out the 68020 forms GCC otherwise picks ([The build pipeline](build-pipeline.md) covers that experiment).

GCC also calls `memcpy` or `memset` on its own, even under `-ffreestanding`. Compiling a function that copies one `struct big` into another with `*d = *s;` turns the assignment into a `memcpy` call at `-Os`:

```text
54:	4eb9 0000 0000 	jsr 0 <add3>
			56: R_68K_32	memcpy
```

A large zero-initialized struct returned through a pointer triggers `memset` the same way (a `struct big`-sized zero-init just compiles to inline `clrl`):

```text
   a:	4eb9 0000 0000 	jsr 0 <zero_bigger>
			c: R_68K_32	memset
```

A fill or copy loop can become a call too, through GCC's `-ftree-loop-distribute-patterns` pass, enabled at `-Os`: GCC 13.3 for x86-64 turns a byte fill loop into `memset` at `-O2`. With this toolchain, four such loops, byte and word, fixed and variable count, stayed loops at `-Os` and `-O2`. Nothing in the project defines `memcpy` yet, so the struct-copy call fails to link. `memcpy` and `memset` go together; add `memmove` and `memcmp` the same way if a link asks:

```c
#include <stddef.h>

/* GCC calls these for struct copies and array initialisers even when freestanding.
 * Keep them in src/rt/mem.c and add it to CMakeLists.txt when the link asks for them. */
void *memcpy(void *dst, const void *src, size_t n)
{
    unsigned char *d = dst;
    const unsigned char *s = src;
    while (n--) {
        *d++ = *s++;
    }
    return dst;
}

void *memset(void *dst, int c, size_t n)
{
    unsigned char *d = dst;
    while (n--) {
        *d++ = (unsigned char)c;
    }
    return dst;
}
```

64-bit arithmetic and floating point fail the same way. A `long long` division and a `float` multiply compile cleanly at `-c`, then fail to link against nothing:

```text
undefined reference to `__divdi3'
undefined reference to `__mulsf3'
```

`src/rt` supplies only the 32-bit integer helpers; a patch needing 64-bit division or a float multiply must avoid it or add the helper itself. The undefined reference at link, not a compile error, is the signal.

## The calling convention

Arguments go on the stack, right to left, one 4-byte slot each, and the caller pops them after the call. Compiling `add3(uint32_t a, uint32_t b, uint32_t c) { return a + b + c; }` reads all three slots off `%sp`:

```text
   0:	202f 0008      	movel %sp@(8),%d0
   4:	d0af 000c      	addl %sp@(12),%d0
   8:	d0af 0004      	addl %sp@(4),%d0
```

and `call_u16(uint16_t v) { take_u16("x", v); }`, which passes a `uint16_t` argument, zero-extends it into its slot before pushing, then pops both slots after the call returns:

```text
  30:	7000           	moveq #0,%d0
  32:	302f 0006      	movew %sp@(6),%d0
  36:	2f00           	movel %d0,%sp@-
  44:	508f           	addql #8,%sp
```

`moveq #0,%d0` fills the slot's high word with zero before the low word is set, so a callee reading the same argument as `uint16_t` takes it from offset 2 of the 4-byte slot, not offset 0.

An integer result comes back in `d0`. A pointer result comes back in `a0`, copied to `d0` too: `ptr_ret(char *p) { return p + 1; }` computes in `a0` and ends `movel %a0,%d0`. A struct result is written through a pointer the caller passes in `a1`; `struct_ret` starts

```text
  18:	2049           	moveal %a1,%a0
```

copying `a1` into `a0` before writing the result through it, so the callee returns the same pointer in `a0` too.

`d0`, `d1`, `a0`, and `a1` are caller-saved: `struct_ret` alone uses all four as scratch, computing two fields in `d0` and `d1`, writing through `a1`, and returning the result in `a0`, none of them saved. GCC's m68k convention treats `d2` to `d7` and `a2` to `a6` as callee-saved; compiled with `-fomit-frame-pointer` so `a6` is free too, a function keeping one live value per register across a call saves and restores the whole set in one instruction each way:

```text
   0:	48e7 3f3e      	moveml %d2-%d7/%a2-%fp,%sp@-
  7a:	4cdf 7cfc      	moveml %sp@+,%d2-%d7/%a2-%fp
```

(objdump names `a6` `%fp`; the range is exactly the eleven registers claimed.)

`a7` is the stack pointer; `a6` is free the same way only when `-fomit-frame-pointer` is set, as `cmake/m68k-toolchain.cmake` does, otherwise it holds the frame pointer.

## Sizes and alignment

Every size and alignment claim on this page is one line of this snippet, which the check compiles:

```c
#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>
#include <limits.h>
#include <stdarg.h>

/* Every line here is checked by the compiler; if one fails, this page is wrong. */
_Static_assert(sizeof(char) == 1, "");
_Static_assert(sizeof(short) == 2, "");
_Static_assert(sizeof(int) == 4, "");
_Static_assert(sizeof(long) == 4, "");
_Static_assert(sizeof(long long) == 8, "");
_Static_assert(sizeof(void *) == 4, "");
_Static_assert(_Alignof(int) == 2, "");
_Static_assert(_Alignof(long long) == 2, "");
_Static_assert(_Alignof(void *) == 2, "");

struct pair { uint8_t tag; uint32_t value; };
_Static_assert(sizeof(struct pair) == 6, "one byte of padding after tag; 32-bit values need only even alignment");
```

A word or long-word access at an odd address raises an address error, vector `0x00C` ([The ROM image](rom-image.md)), per the M68000 Family Programmer's Reference Manual. A pointer built from a packed byte offset can land on an odd address; reading the same bytes through a `uint8_t` array avoids the trap, since a byte access needs no alignment:

```c nocheck
uint32_t v = *(uint32_t *)(base + 1);   /* traps if base is even: odd address */

uint8_t b[4];
uint32_t safe = ((uint32_t)b[0] << 24) | (b[1] << 16) | (b[2] << 8) | b[3];
```

## Hardware and the game's memory

A hardware register or a location in the game's RAM must be read and written through `volatile`, or the compiler may keep a stale copy instead of issuing the bus access:

```c
#include <stdint.h>

#define VDP_CTRL (*(volatile uint16_t *)0xC00004)

/* volatile makes every read a bus access; without it the compiler may keep a stale copy. */
uint16_t vdp_status(void)
{
    return VDP_CTRL;
}
```

The 68000 is big-endian, the same order the ROM's own checksum is computed in ([The ROM image](rom-image.md)), so a `uint16_t` or `uint32_t` cast over a hardware port or a game structure needs no byte-swapping.

## Game symbols

`game/<name>.h` and `game/<name>.ld` (`<name>` from `mdrom init`) hold a game symbol's two halves: the C declaration and the address a linker symbol binds it to ([Project layout](project-layout.md)). A variable or routine taking no register arguments can be declared `extern` and called directly:

```c
#include <stdint.h>

/* Declarations as they would appear in game/<name>.h; the .ld file binds the addresses:
 *   game_party_gold = 0xFF1234;
 *   game_show_menu  = 0xA700;
 * The routine takes no register arguments, so C can call it directly. */
extern volatile uint16_t game_party_gold;
void game_show_menu(void);

void double_gold(void)
{
    game_party_gold *= 2;
    game_show_menu();
}
```

`0xFF1234` and `0xA700` are sample addresses, not facts about Warriors of the Eternal Sun; the routine takes no register arguments, so C calls it directly. A register-interface routine, such as `game_draw_text` on [Writing assembly](writing-asm.md), takes arguments or a return value in registers, not the stack, and needs a matching wrapper ([Call a game routine](recipes/call-a-game-routine.md)).

## Data

`const` data goes into the same output section as code, since `rom.ld` maps `.rodata` there: `.text : { *(.text*) *(.rodata*) } > ROM_FREE` ([The build pipeline](build-pipeline.md)).

```c
#include <stdint.h>

/* const data lands in .rodata, which rom.ld places in ROM_FREE. */
const uint8_t encounter_rates[4] = { 3, 5, 8, 12 };

uint8_t rate(uint8_t zone)
{
    return encounter_rates[zone & 3];
}
```

A writable global needs RAM, which the generated project has not chosen yet. `rom.ld` asserts as much:

```ld
ASSERT(SIZEOF(.data) == 0, "RAM placement is not defined yet: no initialised globals")
ASSERT(SIZEOF(.bss) == 0, "RAM placement is not defined yet: no globals")
```

An initialized or zeroed global fails the link until a RAM range is placed ([Add RAM variables](recipes/add-ram-variables.md)).

## Code generation

`-Os` optimizes for size, the setting `cmake/m68k-toolchain.cmake` picks for every build configuration. Compiling the same arithmetic without `-m68000` emits 68020-only `mulsl` and `divull` in place of the `__mulsi3`/`__udivsi3` calls above ([The build pipeline](build-pipeline.md), which covers that experiment). `m68k-linux-gnu-objdump -dr` read every disassembly on this page; the same tool reads a real build's `patch.elf` too ([The build pipeline](build-pipeline.md)).

A single instruction with no register interface of its own is small enough to inline:

```c
/* One instruction is fine inline; anything with a register interface belongs in a .s file. */
static inline void mask_interrupts(void)
{
    __asm__ volatile ("move.w #0x2700,%%sr" ::: "memory");
}

void enter_critical(void)
{
    mask_interrupts();
}
```

Anything needing specific registers set up, a call into the game, or more than a line of hand-written encoding belongs in a `.s` file ([Writing assembly](writing-asm.md)).
