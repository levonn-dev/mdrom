    .section .hook_000300
    jsr     poc_value
    .section .orig_000300,""
    .byte   0x4e,0x71,0x4e,0x71,0x4e,0x71
    .section .note.GNU-stack,"",%progbits
