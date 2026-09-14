# How a patch works

A patch built with mdrom starts from a clean Mega Drive ROM and ends as a BPS file. Applying it to the same clean ROM reproduces the patched image byte for byte, so nobody redistributes the ROM. Four commands the build runs carry a project from one end to the other: `mdrom free` measures where new bytes may go, the cross compiler and linker place code there and at fixed hook offsets, `mdrom inject` writes it into the ROM, and `mdrom bps create` turns the result into the file a player applies. A fifth command, run by the player rather than the build, applies it. This page walks that pipeline: why a hook is the only way new code runs, where new code can go, what mdrom refuses to inject, and what its build checks catch first.

## The pipeline

1. Start from a clean ROM. `mdrom init` reads it once to record its size and SHA-1, then writes a CMake project beside it; the ROM itself is untouched.
2. `mdrom free` measures the ROM against the target size and writes a linker script fragment, `free.ld`, defining the `ROM_FREE` memory region. Building a project prints this first:

   ```text
   Measuring free ROM space
   wrote free.ld
   ```

3. The cross compiler and GNU linker compile the patch's C and assembly and place every section. Ordinary code and data go wherever the linker script sends them, constrained to `ROM_FREE`; hook sections go at the ROM offset named in each one, set with one `--section-start` flag per hook. Two checks then run against the linked ELF (see "What the build checks add") and print:

   ```text
   check-68000: patch.elf is 68000-clean
   check-hooks: 1 hooks paired with their .orig_ sections
   ```

4. `mdrom inject` writes every allocated section with file contents into the clean ROM at its load address, checks each hook's `.orig_` twin first, and fixes the ROM's end address and checksum afterward. The same build prints:

   ```text
   Injecting patch.elf into the ROM
   wrote patched.md
   ```

5. `mdrom bps create` diffs the clean and patched ROMs, writes the BPS patch, and applies it in memory against the clean ROM to confirm the result matches before it says so:

   ```text
   Writing patch.bps
   wrote patch.bps (534 bytes, self-check passed)
   ```

6. A player who owns the game applies the patch to their own copy, with the fifth command below.

The four commands the build actually runs, with the flags its templates set:

```sh
mdrom free base.md --size 0x200000 --ld build/m68k/free.ld
m68k-linux-gnu-gcc -m68000 -ffreestanding -nostdlib -fno-pic -fomit-frame-pointer -Os -Wall -Wextra -g \
    -static -Wl,--build-id=none -Wl,-z,noexecstack -Wl,--entry=0 \
    -Xlinker -L -Xlinker build/m68k -Xlinker -L -Xlinker game -Xlinker -T -Xlinker link/rom.ld \
    -Xlinker -Map -Xlinker build/m68k/patch.map -Wl,--section-start=.hook_000004=0x000004 \
    src/patch.c src/rt/div.c src/rt/mul.s hooks/hooks.s -o patch.elf
mdrom inject --base base.md --elf patch.elf --out patched.md --size 0x200000 \
    --expect-sha1 9135f7fda03ef7da92dfade9c0df75808214f693
mdrom bps create --base base.md --target patched.md --out patch.bps
```

`-g` never changes the ROM bytes; it only adds debug symbols. The SHA-1 is the Warriors of the Eternal Sun ROM used throughout this guide, and `0x000004` is the project's one hook, on the reset vector.

The fifth command, the player's own, never run by the build:

```sh
mdrom bps apply --base base.md --patch patch.bps --out game.md
```

## Why hooks

A Mega Drive game runs from ROM, and every instruction it executes and every byte it reads sits at a fixed address before a patch touches anything. mdrom cannot make the game run code nothing in it ever reaches, so the only way new code runs is to overwrite bytes the game already executes or reads with something that leads to the new code: a jump, a subroutine call, or a data word loaded as an address. That overwritten spot is a hook: the only original bytes anywhere in the ROM that a patch built with mdrom changes. Everything else mdrom writes lands where nothing was before it (see "Where new code goes").

Because a hook overwrites something, mdrom needs to know what was there before, to check it lands on the ROM revision its author expects. That is what the `.orig_XXXXXX` section records: the clean-ROM bytes at the same offset and size as its `.hook_XXXXXX` twin, checked against the base ROM before any byte is written (see "What mdrom refuses").

## Where new code goes

Anything that is not a hook has no fixed address to go to; it must land where `mdrom free` already confirmed nothing else lives. Free space comes in three kinds. Expansion is the bytes beyond the ROM's original length, up to the requested size. Tail padding is unused bytes at the very end of the original ROM. Both are always usable, and `mdrom free` merges them into one linker `MEMORY` region, `ROM_FREE`. For the Warriors of the Eternal Sun ROM (1,048,576 bytes) built to `--size 0x200000`, `mdrom free` wrote:

```text
ROM_FREE (rx) : ORIGIN = 0x0FFFF0, LENGTH = 0x100010
```

Sixteen of those bytes, at the front, are tail padding inside the 1MB ROM; the rest is expansion past its original end. `ROM_FREE` is declared `(rx)`, not a catch-all default, so a linker script should place every output section into it, or another named region, explicitly. GNU ld keeps an unplaced section as an orphan rather than discarding it, though, and an allocatable orphan whose flags match `ROM_FREE`'s `(rx)` attributes is placed there by the linker anyway, so it does reach the image; `mdrom inject` writes every allocated section with file contents regardless, as build-pipeline.md's "The linker script" section, linked below, covers in full.

The third kind, interior filler, is a run of repeated bytes found inside the ROM rather than at its edges. `mdrom free` reports these runs but excludes them from `ROM_FREE` unless a build passes `--interior`, because in many ROMs a run that looks like padding is a blank data table the game reads, not space nobody uses. Passing `--interior` is a claim that a particular run really is spare.

Full detail on the linker script, every region it defines, and the flags above is in [The build pipeline](build-pipeline.md). The address ranges a hook or a new section can safely target, and what lives at each one in a stock Mega Drive, is in [The ROM image](rom-image.md).

## What mdrom refuses

`mdrom inject` fails before writing any byte in five situations, each shown below from a small ELF built to trigger it against the Warriors of the Eternal Sun ROM.

A hook not at its offset: a `.hook_XXXXXX` section must be linked at exactly the address named in its six hex digits. Linking the reset-vector hook one word too high produces:

```text
.hook_000004 is linked at 0x000006, not 0x000004; add -Wl,--section-start=.hook_000004=0x000004 to the link
```

The name is the only place mdrom learns where a hook belongs. If the link disagrees with the name, mdrom refuses rather than guess which is right.

An `.orig_` mismatch: mdrom compares each hook's `.orig_XXXXXX` twin against the base ROM first. Corrupting one byte of the clean ROM at that offset produces:

```text
.orig_000004: base ROM has FF000200 at 0x000004, expected 00000200
```

`00000200` is the real reset vector of the clean ROM; a mismatch means the base ROM is not the revision the patch was built against, or the offset is wrong, and either way the hook's assumption about what it overwrites no longer holds.

A hook without its twin: that comparison needs a same-size `.orig_XXXXXX` section. Linking the reset-vector hook without `.orig_000004` produces:

```text
.hook_000004 has no .orig_000004 twin recording the bytes it overwrites; pass --allow-unverified-hook to inject it unchecked
```

`--allow-unverified-hook` turns that into a warning. An `.orig_` with no hook at its offset, or of another size, is refused either way.

A non-hook section outside free space: only a hook may overwrite existing bytes. Linking an ordinary `.text` section at an address `mdrom free` never reported free produces:

```text
.text at 0x000100-0x000102 would overwrite non-free byte 0x000100; pass --allow-overwrite if that is intended
```

mdrom will not clobber bytes nobody confirmed are unused unless the user says overwriting them is intended.

A wrong SHA-1: `--expect-sha1` names the exact ROM revision a build was made for. Passing a value that is not the clean ROM's own hash produces:

```text
base ROM SHA-1 is 9135f7fda03ef7da92dfade9c0df75808214f693, expected deadbeefdeadbeefdeadbeefdeadbeefdeadbeef
```

Every check above assumes the base ROM is the one the patch was built against; catching the wrong revision here stops anything downstream running against it by mistake.

## What the build checks add

Two checks run against the linked ELF before `mdrom inject` ever sees it. The cross compiler defaults to the 68020, and the toolchain file passes `-m68000` to every compile to stop that, but a normal link also pulls in libgcc for helpers like 32-bit multiply and divide, and Ubuntu's prebuilt libgcc targets the 68020. The generated project links with `-nostdlib` and supplies its own helpers instead, so `check-68000.sh` disassembles the linked ELF and fails if it finds any instruction or addressing mode the 68000 lacks, catching a stray 68020 object before it reaches a ROM. The output shown above, `check-68000: patch.elf is 68000-clean`, is that check passing.

`check-hooks.sh` repeats mdrom's pairing check at link time and adds what mdrom cannot see: every hook offset in the project's `CMakeLists.txt` must be linked, and every linked hook must be listed there. Its output above, `check-hooks: 1 hooks paired with their .orig_ sections`, confirms the generated project's hook has its twin.

Both checks, along with the rest of what CMake runs during a build, are covered in [The build pipeline](build-pipeline.md).
