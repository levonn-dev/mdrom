| 32-bit multiply helper GCC calls on the 68000, which has only 16x16 mulu.
| Same algorithm as libgcc's: three partial products, with the cross terms
| added into the upper word. Arguments on the stack, result in d0.
    .text
    .globl  __mulsi3
__mulsi3:
    move.w  4(%sp),%d0      | a.hi
    mulu.w  10(%sp),%d0     | a.hi * b.lo
    move.w  6(%sp),%d1      | a.lo
    mulu.w  8(%sp),%d1      | a.lo * b.hi
    add.w   %d1,%d0
    swap    %d0
    clr.w   %d0
    move.w  6(%sp),%d1      | a.lo
    mulu.w  10(%sp),%d1     | a.lo * b.lo
    add.l   %d1,%d0
    rts
    .section .note.GNU-stack,"",%progbits
