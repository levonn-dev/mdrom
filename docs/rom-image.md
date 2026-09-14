# The ROM image

A Mega Drive ROM is bytes the 68000 addresses directly: no loader relocates it, no page table sits between an instruction fetch and the chip on the cartridge board. Every fact mdrom relies on to patch an image, the CPU's exception vectors, the cartridge header it reads and rewrites, the checksum it recomputes, lives at a fixed offset in that flat address space. This page maps the address ranges, the vector table and header at the start of the image, what happens at reset before a game's own code runs, and where mdrom's size limits come from.

## The 68000's address space

| Range | Contents |
|---|---|
| 0x000000-0x3FFFFF | Cartridge ROM |
| 0xA00000-0xA0FFFF | Z80 address space |
| 0xA10000-0xA1001F | I/O registers (version register at 0xA10001) |
| 0xA14000 | TMSS register |
| 0xC00000 | VDP data port |
| 0xC00004 | VDP control port |
| 0xC00008 | HV counter |
| 0xC00011 | PSG |
| 0xFF0000-0xFFFFFF | Work RAM, 64KB, mirrored from 0xE00000 |

Source: Sega's Mega Drive/Genesis software development manual.

Reading an address in a disassembly means placing it in one of these ranges first: an instruction that reads or writes 0xC00004 talks to the VDP, not RAM. The WOTES ROM's entry point shows this: its first instruction, at the reset vector target 0x200, is `tst.l 0xA10008`, a read from the I/O range, not cartridge space.

## The vector table

| Offset | Vector |
|---|---|
| 0x000 | Initial stack pointer |
| 0x004 | Initial program counter (reset) |
| 0x008 | Bus error |
| 0x00C | Address error |
| 0x010 | Illegal instruction |
| 0x014 | Zero divide |
| 0x018 | CHK instruction |
| 0x01C | TRAPV instruction |
| 0x020 | Privilege violation |
| 0x024 | Trace |
| 0x028 | Line 1010 emulator |
| 0x02C | Line 1111 emulator |
| 0x060 | Spurious interrupt |
| 0x064-0x07C | Interrupt autovectors, level 1 to 7 |
| 0x080-0x0BC | TRAP #0 to #15 |

Source: the M68000 Family Programmer's Reference Manual, exception vector assignments.

On the Mega Drive, three of the seven autovector levels matter: level 2 (0x068) is the external interrupt, level 4 (0x070) the horizontal blank, and level 6 (0x078) the vertical blank (Sega's manual). The table is 64 longwords, 0x000 to 0x0FF, which is why the header starts at 0x100 and code at 0x200. Every entry above is a 4-byte address, not an instruction, so a patch can overwrite it like any other ROM byte: replace the pointer at 0x078 and the game's own handler never runs again. The generated project's one hook does exactly this at 0x004, the reset vector: see [Hooks](hooks.md).

## The header

| Offset | Field | Size | mdrom |
|---|---|---|---|
| 0x100 | System name | 16 | info |
| 0x110 | Copyright | 16 | info |
| 0x120 | Domestic title | 48 | info |
| 0x150 | Overseas title | 48 | info |
| 0x180 | Serial | 14 | info |
| 0x18E | Checksum | 2 | info, inject |
| 0x190 | Device support | 16 | - |
| 0x1A0 | ROM start | 4 | info |
| 0x1A4 | ROM end | 4 | info, inject |
| 0x1A8 | RAM start | 4 | info |
| 0x1AC | RAM end | 4 | info |
| 0x1B0 | SRAM flag and type | 4 | info |
| 0x1B4 | SRAM start | 4 | info |
| 0x1B8 | SRAM end | 4 | info |
| 0x1F0 | Region | 3 | info |

Source: `internal/rom/header.go`, the offsets `mdrom info` and `mdrom inject` read and write. The 16-byte gap at 0x190 sits between fields the code parses, not one of them: header.go has no offset constant for it. `mdrom info` prints every other row; `mdrom inject` rewrites two, ROM end and checksum, after writing a build's sections into the image. The checksum is the 16-bit sum of big-endian words from 0x200 (`HeaderSize` in the same file) to the last byte of the image. The region code is 3 bytes, one letter per region, space padded; the 13 bytes after it are reserved (Plutiedev's ROM header reference) and, like the gap at 0x190, not parsed.

Running `mdrom info` against the clean ROM and against a build injected to a 2MB target shows what changes and what does not (the clean ROM's `file:` path is shortened to its filename below):

```text
file:      Dungeons & Dragons - Warriors of the Eternal Sun (USA, Europe).md
size:      1048576 (0x100000)
sha1:      9135f7fda03ef7da92dfade9c0df75808214f693
system:    SEGA GENESIS
copyright: (C)T-50 1992.APR
domestic:  WARRIORS OF THE ETERNAL SUN
overseas:  WARRIORS OF THE ETERNAL SUN
serial:    GM MK-1304 -00
rom:       0x000000-0x0FFFFF
ram:       0xFF0000-0xFFFFFF
sram:      type 0xF8 at 0x200001-0x203FFF
region:    U
checksum:  stored 0x90C4 computed 0x90C4 OK
```

```text
file:      build/m68k/patched.md
size:      2097152 (0x200000)
sha1:      5fa5be75213bb5023580a09896307d44fe0e808a
system:    SEGA GENESIS
copyright: (C)T-50 1992.APR
domestic:  WARRIORS OF THE ETERNAL SUN
overseas:  WARRIORS OF THE ETERNAL SUN
serial:    GM MK-1304 -00
rom:       0x000000-0x1FFFFF
ram:       0xFF0000-0xFFFFFF
sram:      type 0xF8 at 0x200001-0x203FFF
region:    U
checksum:  stored 0x2A15 computed 0x2A15 OK
```

Size doubles from 1,048,576 to 2,097,152 bytes and `rom` grows from `0x0FFFFF` to `0x1FFFFF` to match; every text field and the `ram` and `sram` ranges stay identical; `sha1` changes because injection changed the bytes; the checksum changes from `0x90C4` to `0x2A15` because `mdrom inject` recomputed it, and both report `OK` because each matches its own image. Sega's manual documents this checksum so a game can check it at boot and refuse to continue, or warn, on a mismatch.

## Reset and TMSS

At reset the 68000 loads its stack pointer from address 0 and program counter from address 4, and sets the status register to supervisor mode with interrupts masked before fetching anything else (M68000 Family Programmer's Reference Manual). Those two addresses are entries in the vector table above, and for the WOTES ROM `xxd -l 16` on the clean image reads `ffff fff6 0000 0200 0000 0200 0000 0200`: an initial stack pointer of `0xFFFFFFF6` and program counter of `0x00000200`, the address the generated project's reset-vector hook overwrites.

A console with the TMSS boot check does not bypass this: it runs its own short routine first, reads the four bytes at the header's 0x100 for `SEGA`, and only then loads the cartridge's own vectors from 0 and 4, same as a console without the check. Software running under TMSS must also write `SEGA` to the register at 0xA14000 before touching the VDP whenever the I/O version register's low four bits are non-zero; skipping that write leaves the VDP inaccessible. BlastEm's `tmss.md`, a ROM the emulator ships to exercise this path, and TMSS boot ROM disassembly write-ups covering the same check are the sources for both claims.

## Image sizes

The cartridge range Sega's manual gives, 0x000000 to 0x3FFFFF, is 4MB: the largest image a cartridge without bank-switching hardware can present in one piece. `mdrom free` and `mdrom inject` default `--max-size` to that same 0x400000, raised only for a cart with a mapper. `--size` pads a build up to a requested length; the worked examples here use a power of two for it, the WOTES ROM's own 0x100000 built to a 0x200000 target, because a cartridge without a mapper decodes its address lines by mirroring an image that does not fill its power-of-two window, repeating the same bytes at higher addresses instead of leaving them unmapped. A non-power-of-two `--size` does not fail either command, only warns: `warning: size N is not a power of two; some emulators and flash carts assume it`.

## Where new code goes

Everywhere in the image outside a named hook is bytes the game already reads or executes, and mdrom leaves it untouched at injection except at the offsets a `.hook_XXXXXX` section names. Free bytes it may write come in tail padding and expansion, the two kinds `mdrom free` merges by default into one linker region, `ROM_FREE`; a third, interior filler, is reported but excluded unless `--interior` is passed (see [How a patch works](how-a-patch-works.md)). Run at the clean ROM's own size, `mdrom free` reports one tail run, `0x0FFFF0 0x100000 0x000010 0xFF tail`: 16 bytes of 0xFF padding at the end of the original image, so the first megabyte of any build changes only there. Run again with `--size 0x200000`, it adds an expansion run, `0x100000 0x200000 0x100000 0xFF expansion`, from the original length to the build size, and reports `usable: 1048592 bytes at 0x0FFFF0 (tail + expansion)`, the two merged into one region.

The rest of a region stays byte-identical to the clean ROM because `mdrom inject` only writes the sections it is given, plus the ROM end and checksum, not because of any check. The `.orig_XXXXXX` twin checks something else, a hook's target bytes against what its author expects there; how-a-patch-works.md's "What mdrom refuses", linked above, covers it. Keeping a region byte-identical when that matters, tail padding included, takes a `--size` equal to the clean ROM's length so nothing expands, or an edited `free.ld` `ORIGIN` starting where expansion begins, leaving the tail outside `ROM_FREE`.
