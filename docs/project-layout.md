# Project layout

`mdrom init` writes fifteen files into a new directory beside the clean ROM: a CMake project with a cross toolchain, a linker script, a reset-vector hook, the runtime helpers a freestanding 68000 build needs, and the checks that run against every link. This page describes each file, the outputs a build produces, where to make each kind of change, and the two commands that configure and build the project.

## The generated files

`<name>` in the table below is the project's name, chosen by `mdrom init` or given with `--name`; this guide's examples name it `wotes`, for the Warriors of the Eternal Sun ROM.

| File | Purpose | What depends on it |
|---|---|---|
| `CMakeLists.txt` | Declares the project, the ROM path and size, the `HOOKS` list, and every build step: `free.ld` generation, `patch.elf`'s sources and link flags, the post-build checks, `mdrom inject`, and `mdrom bps create`. | The four sources in `add_executable` (`src/patch.c`, `src/rt/div.c`, `src/rt/mul.s`, `hooks/hooks.s`), the `game` include path, the three `LINK_DEPENDS` entries (`link/rom.ld`, `game/<name>.ld`, `build/m68k/free.ld`), `cmake/mdrom.cmake` through `include()`, and `scripts/check-68000.sh`, `scripts/check-hooks.sh`, and `scripts/check-rt.sh`, run as commands. |
| `CMakePresets.json` | Defines the `m68k` configure and build presets: a Ninja build in `build/m68k` using `cmake/m68k-toolchain.cmake`. | The two commands in "Configuring and building" below, which name the `m68k` preset. |
| `cmake/m68k-toolchain.cmake` | Sets the cross compiler (`m68k-linux-gnu-gcc`), the compile flags (`-m68000 -ffreestanding -nostdlib -fno-pic -fomit-frame-pointer -Os -Wall -Wextra -g` for C, `-m68000 -g` for assembly), the linker flags (`-m68000 -nostdlib -static -Wl,--build-id=none -Wl,-z,noexecstack -Wl,--entry=0`), and clears the per-build-type flag variables so `Debug` and `Release` compile identically. | `CMakePresets.json`'s `toolchainFile`. |
| `cmake/mdrom.cmake` | Finds the `mdrom` binary, in `tools/mdrom/`, `$(go env GOPATH)/bin`, `~/go/bin`, or `PATH`, and sets it as `MDROM`; fails the configure if none is found. | `CMakeLists.txt`'s `include(cmake/mdrom.cmake)`, then every `${MDROM}` command the rest of the file runs. |
| `game/<name>.h` | Prototypes for original game routines named in `<name>.ld`; empty until routines are mapped. | `src/patch.c`, which includes it. |
| `game/<name>.ld` | Addresses of original game routines and RAM as linker symbols; empty until they are mapped. | `link/rom.ld`'s `INCLUDE <name>.ld`. |
| `hooks/hooks.s` | The reset-vector hook: `.hook_000004` holds a pointer to `patch_entry`, which calls `patch_init` and then jumps to the original entry point as an immediate address; `.orig_000004` holds that same address separately, for `mdrom inject`'s check against the base ROM before it writes anything. | `CMakeLists.txt`'s `HOOKS` list (`000004`) and the `--section-start` flag it generates; `src/patch.c`'s `patch_init`, which it calls. |
| `link/rom.ld` | Includes `free.ld` (generated into the build directory) and `game/<name>.ld`, found through the `-L` paths `CMakeLists.txt` sets before `-T`; places `.text`, `.rodata`, `.data`, and `.bss` into `ROM_FREE`; `ASSERT`s that `.data` and `.bss` stay empty until RAM placement is defined. | `patch.elf`'s link step: `CMakeLists.txt` lists it in `LINK_DEPENDS` and passes it with `-T`. |
| `scripts/check-68000.sh` | Disassembles `patch.elf` and fails if it finds an instruction the 68000 lacks. | `CMakeLists.txt`'s `POST_BUILD` step, which runs it against every link. |
| `scripts/check-hooks.sh` | Confirms every `.hook_XXXXXX` section named in `HOOKS` has a same-size `.orig_XXXXXX` twin. | The same `POST_BUILD` step, which passes it the `HOOKS` list as arguments. |
| `scripts/check-rt.sh` | Compiles `tests/rt_test.c` against `src/rt/div.c` with the host `gcc` and runs the result. | `CMakeLists.txt`'s `check-rt` target, built as part of `ALL`. |
| `src/patch.c` | `patch_init`, the self-test the reset hook calls: exercises the multiply, divide, and modulus helpers and returns a fixed value; a patch's real startup work replaces this placeholder. | `hooks/hooks.s`, which calls it. |
| `src/rt/div.c` | `__udivsi3`, `__umodsi3`, `__divsi3`, and `__modsi3`, replacing the 68020-only libgcc versions GCC would otherwise call for a 32-bit divide or modulus. | Any compiled code that divides or takes a modulus of a 32-bit value, including `src/patch.c`; checked against native arithmetic by `tests/rt_test.c`. |
| `src/rt/mul.s` | `__mulsi3`, the 32-bit multiply helper the 68000 needs because its only hardware multiply is 16x16. | Any compiled code that multiplies two 32-bit values, including `src/patch.c`. |
| `tests/rt_test.c` | Compares `src/rt/div.c`'s helpers against the host's native division and modulus over a table of edge-case operands. | `scripts/check-rt.sh`, which compiles and runs it. |

## Build outputs

A build writes into `build/m68k`, the directory `CMakePresets.json` names:

- `free.ld`: the `mdrom free` output, a `MEMORY` block defining `ROM_FREE`. `CMakeLists.txt` regenerates it whenever the clean ROM or `ROM_SIZE` changes, so editing it by hand has no lasting effect. For the Warriors of the Eternal Sun ROM (SHA-1 `9135f7fda03ef7da92dfade9c0df75808214f693`) built to `--size 0x200000`, it reads:

```text
/* Generated by mdrom free. Do not edit. */
MEMORY
{
  ROM_FREE (rx) : ORIGIN = 0x0FFFF0, LENGTH = 0x100010
}
```

- `patch.elf`: the linked ELF. The toolchain file's `-g` flag gives it DWARF debug sections (`.debug_info`, `.debug_line`, and others) and leaves it unstripped, so a debugger resolves symbols and line numbers; `-g` adds no bytes to a section placed in `ROM_FREE` and never changes an injected ROM byte.
- `patch.map`: the linker map `-Map` writes, listing the memory configuration (`ROM_FREE`'s origin and length) and every section's placement. In the self-test build, `patch_init` lands at `0x000ffff0` and `__udivsi3` at `0x001000da`.
- `patched.md`: the ROM `mdrom inject` writes, padded to `ROM_SIZE`; 2,097,152 bytes for the `0x200000` build.
- `patch.bps`: the patch `mdrom bps create` writes and verifies by applying it in memory first; 534 bytes for the self-test build.
- `host/rt_test`: the binary `scripts/check-rt.sh` builds with the host `gcc` from `tests/rt_test.c` and `src/rt/div.c`, and runs during the build; it prints `rt_test: 256 pairs ok` when every sampled pair agrees with native arithmetic.

## Where to make each change

- Add a source file: `add_executable(patch.elf ...)` in `CMakeLists.txt` names sources one by one, not by glob, so a new `.c` or `.s` file must be added there too.
- Add a hook: append its six-digit ROM offset to `CMakeLists.txt`'s `HOOKS` list, which both generates the matching `--section-start` flag and is passed to `check-hooks.sh`, then add the `.hook_XXXXXX`/`.orig_XXXXXX` section pair to `hooks/hooks.s`. [Hooks](hooks.md) covers what a hook site may safely do.
- Name a game symbol: add the address to `game/<name>.ld` as a linker symbol (`game_draw_text = 0xA646;`), and its prototype to `game/<name>.h` if C code calls it. [Game symbols](game-symbols.md) covers finding the address.
- Change the ROM size: edit the `set(ROM_SIZE ...)` line in `CMakeLists.txt`. It is the `--size` both `mdrom free` and `mdrom inject` run with, and the two must agree or the addresses `free.ld` defines stop matching the image `inject` produces.
- `-DCLEAN_ROM=<path>`: overrides the `CLEAN_ROM` cache variable, for a ROM at a different path or under a different name than `mdrom init` found.
- `-DMDROM=<path>`: overrides where `cmake/mdrom.cmake` looks for the `mdrom` binary, for an install outside `tools/mdrom/`, `$(go env GOPATH)/bin`, `~/go/bin`, and `PATH`.

## Configuring and building

```sh
cmake --preset m68k
cmake --build --preset m68k
```

The first command configures `build/m68k` with `cmake/m68k-toolchain.cmake`; the second builds `patch.elf`, runs `check-rt`, `check-68000.sh`, and `check-hooks.sh` against it, then `mdrom inject` and `mdrom bps create` through the `rom` target, built by default. CMake reconfigures itself when `CMakeLists.txt` or another listed dependency changes, so the configure command rarely needs repeating by hand. Deleting `build/` and running both commands again starts the project from nothing. Every flag both commands set is in [The build pipeline](build-pipeline.md).
