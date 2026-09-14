# Add a data table

## Goal

Add a new data table in C and point an existing instruction's address operand at it, leaving the instruction itself untouched.

## What you need first

Replacing only the address bytes an instruction reads, without touching flow, is the "Data" hook site in [Hooks](../hooks.md); where a hook pair goes and the `CMakeLists.txt` list it must be added to are in [Project layout](../project-layout.md). A `const` table's placement in `.rodata`, folded into the same output section as code, is [Writing C](../writing-c.md), "Data". Finding the instruction and reading its operand from a disassembly is [Game symbols](../game-symbols.md); the absolute addressing forms an instruction can take are in [Writing assembly](../writing-asm.md).

## Confirming the encoding

An absolute-long operand is a fixed 4-byte address following the instruction's own opcode word. Assembling `lea (0x02A100).l,%a0` alone and disassembling the result shows where those bytes start:

```sh
cat > lea.s <<'S'
    .text
    lea     (0x02A100).l,%a0
S
m68k-linux-gnu-gcc -m68000 -c lea.s -o lea.o && m68k-linux-gnu-objdump -d lea.o
```

```text
   0:	41f9 0002 a100 	lea 0x2a100,%a0
```

`41f9` is the opcode and addressing-mode word, two bytes; `0002 a100` is the absolute long address after it, so the four address bytes start two bytes into the instruction. Overwriting just those four bytes changes only the address `lea` loads; the instruction's opcode and mode stay exactly what the game shipped with. `0x004E20` for the instruction and `0x02A100` for the table it addresses are a sample pair, not real offsets in Warriors of the Eternal Sun; find the real pair from a disassembly before applying this recipe.

## The hook

```asm
| Points the game at a new table: the four address bytes of "lea (0x02A100).l,%a0"
| at 0x004E20 (sample) become the address of new_rates, defined in C.
    .section .hook_004E22,"a"
    .long   new_rates
    .section .orig_004E22,""
    .long   0x0002A100
```

`0x004E22` is `0x004E20` plus the two bytes the opcode word occupies, the offset where the address itself begins. `.orig_004E22` records the table's old address, `0x0002A100`, for mdrom's check against the clean ROM before it writes anything. Add the pair to `hooks/hooks.s` and append `004E22` to `CMakeLists.txt`'s `HOOKS` list.

## The C

```c
#include <stdint.h>

/* The replacement table, same layout as the original the game read at its old address. */
const uint8_t new_rates[8] = { 3, 5, 8, 12, 12, 8, 5, 3 };
```

`new_rates` is `const`, so it lands in `.rodata`, folded into `ROM_FREE` alongside code rather than needing RAM ([Writing C](../writing-c.md), "Data"). Where the linker actually places it does not matter to the hook: `.long new_rates` resolves to whatever address the link assigns, the same as any other symbol reference.

## Build and see it

```sh
cmake --preset m68k
cmake --build --preset m68k
```

`check-hooks.sh` reports the new pair alongside the reset-vector hook, and `mdrom inject` lists `.hook_004E22` among the sections it writes. Once the real site is confirmed, the instruction that used to load the old table's address loads `new_rates`'s instead, and every read through it sees the replacement table without the surrounding code changing at all.
