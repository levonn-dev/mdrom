| Each .hook_XXXXXX section is placed at ROM offset XXXXXX by the HOOKS list in
| CMakeLists.txt; its .orig_ twin holds the clean-ROM bytes mdrom checks before writing.

| Reset vector. The console loads the stack pointer from offset 0 and the program
| counter from offset 4, so patch_entry runs before the game with a valid stack and
| every other register undefined. TMSS consoles take the same vector after their check.
    .section .hook_000004,"a"
    .long   patch_entry
    .section .orig_000004,""
    .long   0x{{printf "%08X" .Entry}}

    .text
    .globl  patch_entry
patch_entry:
    jsr     patch_init
    .globl  patch_ret
patch_ret:
    jmp     0x{{printf "%X" .Entry}}               | the original entry, as in .orig_000004

    .section .note.GNU-stack,"",%progbits
