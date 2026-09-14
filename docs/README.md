# Patching guide

A Mega Drive ROM patch built with mdrom starts from a clean ROM image and reaches the console through a project `mdrom init` writes, a cross-compiled build, and the hook convention that runs new code from otherwise unchanged bytes. Writing one touches C and 68000 assembly, the game's own routines and data, and a GDB session to confirm the result. This guide assumes you know C and 68000 assembly, can work a shell, and have used CMake before. For command flags, see the mdrom README and `mdrom <command> -h`.

## Reading order

1. [How a patch works](how-a-patch-works.md): the build pipeline from a clean ROM to an applied patch, why hooks are the only way new code runs, and what mdrom refuses to inject and why.
2. [The ROM image](rom-image.md): the 68000 address map, the exception vector table, the cartridge header, reset, TMSS, and how image size limits work.
3. [Project layout](project-layout.md): the files `mdrom init` generates, what each one does, and what to edit for each kind of change.
4. [The build pipeline](build-pipeline.md): how CMake runs the build, every compiler and linker flag, the linker script, and the checks that run before injection.
5. [Hooks](hooks.md): the hook convention, the seven kinds of hook site, instruction sizes, and the register, flag, and stack rules a hook must follow.
6. [Writing C](writing-c.md): freestanding C on the 68000, the calling convention, type sizes, volatile hardware access, game symbols, and reading the generated code.
7. [Writing assembly](writing-asm.md): GNU assembler syntax for the 68000, calling C and being called from it, wrapping a game routine, and common mistakes.
8. [Game symbols](game-symbols.md): finding a game's routines, data, and free RAM, and recording each address you find.
9. [Debugging](debugging.md): connecting GDB to BlastEm or MAME, a debugging session against a patch, and a catalogue of common failures.

## Recipes

- [Change a string](recipes/change-a-string.md): replacing a fixed-length string in ROM data with a same-length hook.
- [Run code at boot](recipes/run-code-at-boot.md): growing the generated reset-vector hook, what is not initialized yet, the TMSS write before touching the VDP, and moving to a later site.
- [Replace a routine](recipes/replace-a-routine.md): replacing a whole routine with a jmp hook and an assembly shim for its register interface.
- [Hook mid-routine](recipes/hook-mid-routine.md): hooking into the middle of a routine with a trampoline that preserves and re-executes the displaced instructions.
- [Add a data table](recipes/add-a-data-table.md): adding a data table in C and pointing an existing instruction's address operand at it.
- [Call a game routine](recipes/call-a-game-routine.md): calling an existing game routine from C through a named symbol and an assembly wrapper.
- [Add RAM variables](recipes/add-ram-variables.md): placing new variables in a discovered free RAM range and clearing them at boot instead of using `.data`.

The command reference is the [mdrom README](../README.md) and `mdrom <command> -h`.
