# The build pipeline

CMake turns the four sources `mdrom init` writes into `patch.elf`, then into `patched.md` and `patch.bps`, in nine numbered steps: measuring free space, four compiles, a link, a host-side arithmetic check, and two `mdrom` commands, plus two post-link checks that carry no step number of their own. This page covers every flag the toolchain sets, the linker script, hook placement, link option quoting, the checks, and how to read a build's output.

## What CMake runs

`CMakeLists.txt` wires five things together: a custom command running `mdrom free` that writes `build/m68k/free.ld`; the `patch.elf` executable, built from its four sources, with a `POST_BUILD` step running `check-68000.sh` then `check-hooks.sh` against the link; the `check-rt` target, built as part of `ALL`, comparing the division helpers against the host's own arithmetic; and two custom commands, one running `mdrom inject` to write `patched.md`, the other running `mdrom bps create` to write `patch.bps`, gathered under the `rom` target, also part of `ALL`.

```sh
cmake --build --preset m68k
```

```text
[1/9] Measuring free ROM space
[2/9] Building ASM object CMakeFiles/patch.elf.dir/src/rt/mul.s.obj
[3/9] Building ASM object CMakeFiles/patch.elf.dir/hooks/hooks.s.obj
[4/9] Building C object CMakeFiles/patch.elf.dir/src/patch.c.obj
[5/9] Building C object CMakeFiles/patch.elf.dir/src/rt/div.c.obj
[6/9] Linking C executable patch.elf
check-68000: patch.elf is 68000-clean
check-hooks: 1 hooks paired with their .orig_ sections
[7/9] Injecting patch.elf into the ROM
[8/9] Checking division helpers against native arithmetic
[9/9] Writing patch.bps
```

The two check lines after step 6 carry no bracket: `POST_BUILD` attaches them to the link step instead of counting as one. [Project layout](project-layout.md) lists each generated file; this page covers each step's flags.

## The toolchain flags

`cmake/m68k-toolchain.cmake` sets three flag variables:

```cmake
set(CMAKE_C_FLAGS_INIT "-m68000 -ffreestanding -nostdlib -fno-pic -fomit-frame-pointer -Os -Wall -Wextra -g")
set(CMAKE_ASM_FLAGS_INIT "-m68000 -g")
set(CMAKE_EXE_LINKER_FLAGS_INIT "-m68000 -nostdlib -static -Wl,--build-id=none -Wl,-z,noexecstack -Wl,--entry=0")
```

`-m68000` is mandatory because the compiler defaults to the 68020: compiling `a*b + a/b` for `uint32_t` arguments without it emits

```text
mulsl %d2,%d0
divull %d2,%d1,%d1
```

neither of which exists on a 68000. With `-m68000`, the same code links against relocations for `__mulsi3` and `__udivsi3` instead, the helpers `src/rt/mul.s` and `src/rt/div.c` supply.

`-ffreestanding` tells the compiler not to assume a hosted environment. `-nostdlib` drops the standard startup files and libraries at the link stage: "No libgcc: its prebuilt copy targets the 68020," so `src/rt` supplies `__mulsi3`, `__udivsi3`, `__umodsi3`, `__divsi3`, and `__modsi3` itself. `-fno-pic` disables position-independent code: addresses become absolute or PC-relative, not GOT-indirected. `-fomit-frame-pointer` frees the frame-pointer register. `-Os` optimizes for size. `-Wall -Wextra` add warnings; the toolchain file adds no `-Werror`, so one never fails the build.

`-g` adds DWARF debug sections and, in the toolchain file's own words, "never changes ROM bytes." Stripping `patch.elf` from the Warriors of the Eternal Sun project (SHA-1 `9135f7fda03ef7da92dfade9c0df75808214f693`) and injecting the result confirms it, producing a ROM identical to the debug build's:

```sh
m68k-linux-gnu-objcopy --strip-debug build/m68k/patch.elf stripped.elf
mdrom inject --base base.md --elf stripped.elf --out stripped.md --size 0x200000
cmp stripped.md build/m68k/patched.md && echo identical
```

```text
identical
```

`-static` forces a statically linked binary: `patch.elf` carries two `LOAD` segments and a `GNU_STACK` segment, no `PT_INTERP` or `PT_DYNAMIC`.

`-Wl,--build-id=none` stops the linker from adding a `.note.gnu.build-id` section. Reconfiguring with every linker flag unchanged except the build ID:

```sh
cmake -S . -B build-bid -G Ninja --toolchain cmake/m68k-toolchain.cmake -DMDROM=./mdrom \
  -DCMAKE_EXE_LINKER_FLAGS="-m68000 -nostdlib -static -Wl,--build-id=sha1 -Wl,-z,noexecstack -Wl,--entry=0"
cmake --build build-bid
```

still builds and injects: the note is an ordinary allocatable section with file contents, so it lands, as an orphan, in `ROM_FREE` ahead of `.text`:

```text
.hook_000004         0x000004  4        hook
.note.gnu.build-id   0x0FFFF0  36       ROM_FREE
.text                0x100014  482      ROM_FREE
```

Thirty-six bytes of `ROM_FREE` go to a note the injected ROM never reads. `cmd/mdrom/inject.go` appends the same hint to two errors that can name `.note.gnu.build-id`: the image has no tail padding and no expansion at all, and a section landing outside the padded image; both append "link with -Wl,--build-id=none".

`-Wl,-z,noexecstack` asks the linker for a non-executable stack. The compiler emits an empty `.note.GNU-stack` marker for every C object; an assembly file gets one only if it declares it, which is why this project's `.s` files and every `asm` snippet in the guide end with `.section .note.GNU-stack,"",%progbits`. Dropping `-z noexecstack` from the link still produces the same `GNU_STACK` header, so it only guarantees what already holds. `--entry=0` sets the ELF header's entry point to a literal `0`. Without it, the entry point defaults to wherever the linker places the first section instead, `0x0FFFF0` in this build, an address `mdrom inject` never reads either way: it checks `.orig_` bytes by address, not the ELF header's entry field.

`Debug`, `Release`, `RelWithDebInfo`, and `MinSizeRel` all get their per-configuration flag variables cleared: "Build types must not alter code generation; the flags above are the whole story."

## The linker script

`link/rom.ld` includes `build/m68k/free.ld`, the `MEMORY` block `mdrom free` writes, and `game/<name>.ld`, the game's own symbols, both found through `-L` paths `CMakeLists.txt` puts ahead of `-T`: the linker resolves an `INCLUDE` against `-L` paths it has already seen on the command line, not ones that come later. The excerpt below is the template; the file `mdrom init` generates has the project's name in place of `<name>`.

```ld
INCLUDE free.ld
INCLUDE <name>.ld

SECTIONS
{
  .text : { *(.text*) *(.rodata*) } > ROM_FREE

  .data : { *(.data*) } > ROM_FREE
  .bss (NOLOAD) : { *(.bss*) *(COMMON) } > ROM_FREE
  ASSERT(SIZEOF(.data) == 0, "RAM placement is not defined yet: no initialised globals")
  ASSERT(SIZEOF(.bss) == 0, "RAM placement is not defined yet: no globals")
}
```

Code and read-only data go into `.text`, placed in `ROM_FREE`; `.data` and `.bss` are declared, then asserted empty, since no RAM region is chosen yet. `ROM_FREE` is declared `(rx)`, not a default region, but GNU ld does not discard a section this script fails to place: it keeps the section as an orphan. A non-allocatable orphan links into `patch.elf` regardless of any region, which is why `-g`'s DWARF sections survive every link (see "The toolchain flags"); an allocatable orphan matching a region's attributes lands there instead, as `.note.gnu.build-id` did above. What reaches the injected ROM is decided separately, by `mdrom inject`, which writes every allocated section with file contents (`internal/elfimg/elfimg.go`'s `SHF_ALLOC` and size checks) and refuses one only outside free space.

## Hook placement

Hook sections skip the linker script. `CMakeLists.txt` keeps a `HOOKS` list, `000004` in a generated project, and turns each entry into a `--section-start` flag:

```cmake
foreach(hook IN LISTS HOOKS)
  target_link_options(patch.elf PRIVATE -Wl,--section-start=.hook_${hook}=0x${hook})
endforeach()
```

which produces `-Wl,--section-start=.hook_000004=0x000004` for the reset-vector hook. This is the only place a hook's address comes from; a `.hook_XXXXXX` section linked elsewhere fails mdrom's own check ([How a patch works](how-a-patch-works.md)).

## Link option quoting

The four options setting up the linker's search path, script, and map are wrapped in `SHELL:` and `-Xlinker`:

```cmake
target_link_options(patch.elf PRIVATE
  "SHELL:-Xlinker -L -Xlinker \"${CMAKE_BINARY_DIR}\""
  "SHELL:-Xlinker -L -Xlinker \"${CMAKE_SOURCE_DIR}/game\""
  "SHELL:-Xlinker -T -Xlinker \"${CMAKE_SOURCE_DIR}/link/rom.ld\""
  "SHELL:-Xlinker -Map -Xlinker \"${CMAKE_BINARY_DIR}/patch.map\"")
```

`-Xlinker` keeps each half of a pair, the flag and its path, as one argument, so a checkout under a path with a comma or a space still links: a plain `-Wl,-L,<path>` would split on a comma in it. `SHELL:` stops CMake's option de-duplication from folding the repeated `-Xlinker` tokens across the four options, which would otherwise misalign a flag from its path.

## The checks

`check-68000.sh` disassembles `patch.elf` and fails if it finds "an instruction or addressing mode the 68000 lacks." Its pattern lists 68020-only mnemonics like `mulsl` and `divsl` directly, plus two addressing-mode fragments, `:[248]\)` and `\[`: scaled index and memory indirect addressing are 68020 features with no mnemonic of their own, only different operand syntax.

`check-hooks.sh` checks both directions: every offset in `HOOKS` must have a linked `.hook_XXXXXX` section with a same-size `.orig_XXXXXX` twin, and every `.hook_` or `.orig_` section linked must appear in `HOOKS` and have its twin. Passing an offset nothing links catches the first direction:

```sh
scripts/check-hooks.sh build/m68k/patch.elf 000004 000300
```

```text
check-hooks: HOOKS lists 000300 but no .hook_000300 section was linked
```

exit status 1. `check-rt.sh` builds `tests/rt_test.c` against `src/rt/div.c` with the host's `gcc`, not the cross compiler, and runs the result immediately, comparing every helper against native arithmetic before any of it runs cross-compiled.

## Inject and the patch

The last two steps are `mdrom` commands CMake issues:

```sh
mdrom inject --base base.md --elf patch.elf --out patched.md --size 0x200000 \
    --expect-sha1 9135f7fda03ef7da92dfade9c0df75808214f693
mdrom bps create --base base.md --target patched.md --out patch.bps
```

`--expect-sha1` is the SHA-1 `mdrom init` recorded for the clean ROM; a mismatched base image fails the inject before anything is written. [How a patch works](how-a-patch-works.md) covers every way `inject` can refuse, and what `bps create`'s self-check confirms.

## Reading the result

`patch.map` lists where the linker put everything:

```text
Memory Configuration

Name             Origin             Length             Attributes
ROM_FREE         0x000ffff0         0x00100010         xr

.text           0x000ffff0      0x1e2
 .text          0x000ffff0       0x88 CMakeFiles/patch.elf.dir/src/patch.c.obj
                0x000ffff0                patch_init
 .text          0x00100078      0x12c CMakeFiles/patch.elf.dir/src/rt/div.c.obj
                0x001000da                __udivsi3
```

`readelf -S -W patch.elf` gives section addresses and sizes directly:

```text
  [ 1] .text             PROGBITS        000ffff0 003ff0 0001e2 00  AX  0   0  4
  [ 2] .hook_000004      PROGBITS        00000004 002004 000004 00   A  0   0  1
  [ 9] .orig_000004      PROGBITS        00000000 004ef5 000004 00      0   0  1
```

`objdump -d patch.elf` disassembles it; `patch_entry`, the reset hook's target, opens with a call to `patch_init`:

```text
001001c8 <patch_entry>:
  1001c8:	4eb9 000f fff0 	jsr ffff0 <patch_init>
```

`mdrom info patched.md` reads the ROM's own header, not the ELF:

```text
size:      2097152 (0x200000)
sha1:      5fa5be75213bb5023580a09896307d44fe0e808a
checksum:  stored 0x2A15 computed 0x2A15 OK
```

A matching checksum confirms `mdrom inject` updated the header to match the bytes written.
