# Change a string

## Goal

Replace a fixed-length string in ROM data with one of the same length, so nothing after it shifts.

## What you need first

A data hook only changes bytes a routine reads or displays; it never touches flow ([Hooks](../hooks.md), "Data"). Finding the offset is a byte search over the game's own text ([Game symbols](../game-symbols.md)). The `HOOKS` list and where hook code lives are in [Project layout](../project-layout.md).

## Finding the string

`grep -abo FIGHTER` against the Warriors of the Eternal Sun ROM (SHA-1 `9135f7fda03ef7da92dfade9c0df75808214f693`) prints three offsets:

```sh
grep -abo FIGHTER "Dungeons & Dragons - Warriors of the Eternal Sun (USA, Europe).md"
```

```text
8929:FIGHTER
8982:FIGHTER
892431:FIGHTER
```

`8929` is `0x22E1`, `8982` is `0x2316`. A separate patch project that hooks both offsets records why, in its own hooks file: "Class list on the character-creation screen: FIGHTER becomes WARRIOR in both copies, the plain list at 0x22DA and the menu list with control codes at 0x230D." The third offset, `892431`, sits somewhere else and needs its own check before touching it.

`xxd` around each address shows what surrounds the text. The plain list, NUL-terminated:

```sh
xxd -s 0x22D8 -l 0x20 "Dungeons & Dragons - Warriors of the Eternal Sun (USA, Europe).md"
```

```text
000022d8: 0000 434c 4552 4943 0046 4947 4854 4552  ..CLERIC.FIGHTER
000022e8: 004d 4147 4943 2d55 5345 5200 5448 4945  .MAGIC-USER.THIE
```

The menu list, a two-byte control code and a NUL between names:

```sh
xxd -s 0x230A -l 0x20 "Dungeons & Dragons - Warriors of the Eternal Sun (USA, Europe).md"
```

```text
0000230a: 4e47 0043 4c45 5249 4301 1b00 4649 4748  NG.CLERIC...FIGH
0000231a: 5445 5201 1b00 4d41 4749 432d 5553 4552  TER...MAGIC-USER
```

`FIGHTER` is seven uppercase ASCII bytes in both copies, followed by a NUL or a control code, never by another letter of the next name: a same-length replacement leaves every neighbor untouched.

## The hook

```asm
| Both copies of the class name on the character-creation screen: the plain list at
| 0x22DA and the menu list with control codes at 0x230D. Same length, or the text
| after it shifts.
    .section .hook_0022E1,"a"
    .ascii  "WARRIOR"
    .section .orig_0022E1,""
    .ascii  "FIGHTER"

    .section .hook_002316,"a"
    .ascii  "WARRIOR"
    .section .orig_002316,""
    .ascii  "FIGHTER"
```

`WARRIOR` and `FIGHTER` are both seven bytes. Add the pair to `hooks/hooks.s` and append both offsets to `CMakeLists.txt`'s `HOOKS` list, which already carries the generated reset-vector hook: `HOOKS` becomes `000004 0022E1 002316`.

## Build and see it

```sh
cmake --preset m68k
cmake --build --preset m68k
```

Building this pair reports:

```text
check-hooks: 3 hooks paired with their .orig_ sections
```

and `mdrom inject` lists `.hook_0022E1` and `.hook_002316` alongside the reset-vector hook among the sections it wrote. Run the patched ROM in an emulator and start a new game: the class list reads WARRIOR instead of FIGHTER.
