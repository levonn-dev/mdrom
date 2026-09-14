# Add RAM variables

## Goal

Reserve a range of the console's work RAM for new variables, once a range is confirmed free, instead of `.data` or an unplaced `.bss`.

## What you need first

Finding a stretch of work RAM a game does not already use takes filling a candidate range, playing, and dumping what changed ([Game symbols](../game-symbols.md), "Finding free work RAM"). Why the generated project asserts `.data` and `.bss` empty until then, and where those lines sit in `link/rom.ld`, is [Writing C](../writing-c.md), "Data".

## The linker script change

Once a free range is confirmed, give `.bss` an explicit address there instead of asserting it empty. `link/rom.ld`'s `.data` line stays exactly as generated, since nothing on this project copies initial values from ROM into RAM at startup; only the `.bss` line and the two `ASSERT`s change:

```ld
  /* Replace the .bss line and the two ASSERTs in link/rom.ld with this once a free RAM range is known; keep .data where it is. */
  .bss 0xFF8000 (NOLOAD) : { __bss_start = .; *(.bss*) *(COMMON) __bss_end = .; }
  ASSERT(SIZEOF(.bss) <= 0x100, "more RAM variables than the free range holds")
  ASSERT(SIZEOF(.data) == 0, "no initialised globals: nothing copies .data from ROM at startup")
```

`0xFF8000` is a sample address, not a confirmed free range in Warriors of the Eternal Sun; find the real one as [Game symbols](../game-symbols.md) describes, and confirm it stays untouched over real play before trusting it. `__bss_start` and `__bss_end` bound whatever the linker places in `.bss`, so code can zero exactly that range without hardcoding its size; the `0x100` bound in the `ASSERT` should match how much of the confirmed-free range the project means to use. `.data` stays asserted empty for the reason the generated project already gives: an initialized global needs a copy from ROM into RAM at startup, and nothing here performs one, so a global still needs a zero initial value and explicit code, not a nonzero initializer, until that changes.

## The C

```c
#include <stdint.h>

/* rom.ld defines these around .bss once RAM is mapped. Nothing else zeroes .bss. */
extern uint8_t __bss_start[], __bss_end[];

uint16_t frame_count;   /* .bss, at the RAM address rom.ld assigns */

void patch_init(void)
{
    for (uint8_t *p = __bss_start; p < __bss_end; p++) {
        *p = 0;
    }
    frame_count = 0;
}
```

`__bss_start` and `__bss_end` are declared as arrays so their addresses, not any bytes stored at them, are what the loop reads; nothing on this console clears `.bss` before `patch_init` runs, unlike a hosted C runtime's usual startup code. The assignment after the loop is redundant once it has run; it is here only to show that a normal write to a `.bss` variable works exactly like a write to any other global.

## Build and see it

```sh
cmake --preset m68k
cmake --build --preset m68k
```

`m68k-linux-gnu-nm build/m68k/patch.elf | grep -E 'bss|frame_count'` confirms the placement:

```text
00ff8004 B __bss_end
00ff8000 B __bss_start
00ff8000 B frame_count
```

`__bss_start` and `frame_count` land at the base address the script named. `__bss_end` marks the top of everything the linker actually placed in `.bss`, which can sit above `frame_count` alone: the other objects linked into `patch.elf` each carry their own empty `.bss` input section, and the linker pads the output section to satisfy their alignment before closing it. `patch_init`'s loop zeroes whatever that range turns out to be, so the extra padding costs nothing and needs no separate accounting.

`.bss` is `NOLOAD`, so it has no file contents for `mdrom inject` to write; building reports it instead:

```text
bss: .bss at 0xFF8000 (not written)
```

The same rule applies to any bss section: `mdrom inject` writes only sections with file contents, and reports the rest.
