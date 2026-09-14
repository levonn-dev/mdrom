# mdrom

A command-line tool for Mega Drive ROM hacking. It measures a ROM's free space, tells the linker where new code may go, injects a linked ELF into the ROM without overwriting game data, fixes the cartridge header, and creates or applies BPS patches. It knows nothing about any particular game.

## Install

```
go install ./cmd/mdrom
```

## Commands

```
mdrom init   <rom> [--name NAME] [--size N]
mdrom info   <rom>
mdrom free   <rom> [--size N] [--max-size N] [--min N] [--interior] [--ld FILE]
mdrom inject --base ROM --elf ELF --out ROM [--size N] [--max-size N] [--interior] [--allow-overwrite] [--allow-unverified-hook] [--expect-sha1 HEX]
mdrom bps create --base ROM --target ROM --out BPS
mdrom bps apply  --base ROM --patch BPS --out ROM
```

Sizes accept decimal or `0x` hex.

`init` writes a CMake project beside the clean ROM: a toolchain file for the Ubuntu `m68k-linux-gnu` GCC, a linker script that places code in the free space `free` measures, a hook on the reset vector that runs a C self-test before the game starts, the runtime helpers GCC needs on a 68000, and the build-time checks. It refuses to overwrite any file it would write. It prints the image size it chose and warns when that size is not a power of two, adds no expansion, or reaches the SRAM range the header declares. The project builds with `cmake --preset m68k && cmake --build --preset m68k` and needs `cmake`, `ninja-build`, `gcc`, and `gcc-m68k-linux-gnu`.

Exit codes: `0` success; `2` when no command or an unknown command is given, or for `-h` on any command; `1` for every other failure, including an unknown flag or a stray argument.

`free` and `inject` must be given the same `--size`: `free` reports `ROM_FREE` against that target size, and `inject` pads the image to it before placing sections, so a mismatch between the two commands makes the addresses `free` reported wrong for the image `inject` actually produces.

## Workflow

1. `mdrom init game.md` writes the project; the steps below are what its build runs, so read on to see what each step does, or skip to building.
2. `mdrom free base.md --size 0x200000 --ld build/free.ld` writes a `MEMORY` block defining `ROM_FREE`, the padding at the end of the ROM merged with the expansion up to the requested size. Your linker script includes it and places sections with `> ROM_FREE`. `ROM_FREE` is declared `(rx)`, not a catch-all default, so every output section in your linker script must be placed in it (or another named region) explicitly. GNU ld does not discard a section left unplaced, though; it keeps the section as an orphan, and an allocatable orphan whose flags match `ROM_FREE`'s `(rx)` attributes still lands there, the way a `.note.gnu.build-id` section does unless the build ID is disabled. `mdrom inject` then writes every allocated section that reaches the image, hooks included, with its file contents.
3. Link your code with GNU ld. Name each hook section `.hook_XXXXXX` (six hex digits, the ROM offset it overwrites), declare it `.section .hook_XXXXXX,"ax"` for code or `.section .hook_XXXXXX,"a"` for a data word such as a vector, so it is allocatable and non-empty, and place it with `--section-start=.hook_XXXXXX=0xXXXXXX`. A hook section must have file contents; a bss (`SHT_NOBITS`) section named `.hook_XXXXXX` is rejected. Add a non-alloc `.orig_XXXXXX` twin of the same size holding the bytes you expect the clean ROM to have there; `inject` refuses a hook without one unless `--allow-unverified-hook` is passed, and always refuses an `.orig_` section with no hook at its offset or of a different size.
4. `mdrom inject --base base.md --elf patch.elf --out patched.md --size 0x200000` writes every allocated section that has file contents (hooks included; NOBITS/bss sections are reported but never written) at its load address. It fails if a hook is not at the address in its name, if a hook section is not allocatable or is empty or bss, if a hook has no `.orig_` twin (a warning instead with `--allow-unverified-hook`), if an `.orig_` section has no hook at its offset or is not the same size as it, if an `.orig_` declaration does not match the clean ROM, or if any non-hook section would land outside free space. It then fixes the ROM end address and checksum. When the image has no tail padding or expansion, hooks still inject; a non-hook section then fails unless `--interior` makes an interior gap available or `--allow-overwrite` accepts clobbering existing bytes. Both the base image and `--size` must be an even number of bytes.
5. `mdrom bps create --base base.md --target patched.md --out patch.bps` writes the patch and verifies it by applying it in memory first.

Free space comes in three kinds. Expansion (beyond the original length) and tail padding are always usable. Interior filler runs are reported but ignored unless you pass `--interior`, because in many ROMs they are blank data tables rather than padding. The default size ceiling is the 4MB cartridge window; raise `--max-size` only for a cart with a mapper.

## Documentation

The guide in `docs/` explains how a patch works, the project `mdrom init` writes, the build, hooks, and writing the code in C and 68000 assembly, with recipes for the common changes. Start at [docs/README.md](docs/README.md).

## Tests

```
go test ./...
```

Every test runs on synthetic ROM images; no commercial ROM is needed or read. `TestInitBuilds` generates a project from a synthetic ROM and builds it with the real toolchain; it runs only when `m68k-linux-gnu-gcc`, `gcc`, `cmake`, and `ninja` are installed and skips otherwise. CI installs them. The ELF fixtures under `internal/elfimg/testdata/` are rebuilt with `gen.sh`, mostly using the `m68k-linux-gnu-gcc` cross compiler (one fixture, a deliberately non-m68k ELF, uses the host `gcc`).

## Development

[Task](https://taskfile.dev) drives the everyday commands; `task` with no arguments lists them all, and `task check` is the pre-commit gate.

```
task lint          # golangci-lint, formatting included; task lint:fix applies what it can
task test          # go test ./...
task test:cover    # tests with coverage, failing below 80%
task check         # lint + build + test + docs:check
```

CI runs `task lint`, then `task test:cover`, then `task docs:check` after the coverage gate, on every push to `main` and every pull request. The ROM fixture tests skip there; the generated-project build test runs because CI installs the cross tools.

### Prerequisites

Go 1.26, [Task](https://taskfile.dev) 3, and golangci-lint 2.12, which lints and formats in one pass so there is no separate formatter. `task bootstrap` checks that they are installed. GoReleaser is needed only for the `release:*` tasks. golangci-lint installs with the same pinned command CI uses:

```
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/c0d3ddc9cf3faa61a4e378e879ece580256d76e5/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.12.2
```

## Release

Pushing a tag such as `v0.1.0` runs the release workflow: the coverage gate, then GoReleaser publishing `mdrom` binaries for Linux, macOS, and Windows (amd64 and arm64) as a GitHub release with a checksum file. `task release:snapshot` builds the same archives into `dist/` locally without publishing; `task release:check` validates `.goreleaser.yaml`. Both need the folder to be a git repository.
