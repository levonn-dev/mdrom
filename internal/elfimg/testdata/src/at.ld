MEMORY
{
  rom (rx)  : ORIGIN = 0x100000, LENGTH = 0x100000
  ram (rwx) : ORIGIN = 0xFF0000, LENGTH = 0x10000
}
SECTIONS
{
  .text : { *(.text*) *(.rodata*) } > rom
  .data : { *(.data*) } > ram AT> rom
  .bss (NOLOAD) : { *(.bss*) *(COMMON) } > ram
}
