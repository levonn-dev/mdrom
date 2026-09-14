/* Places the patch in the free region mdrom measured. free.ld (generated into the
   build directory) and {{.Name}}.ld (game/) are found through the -L paths CMakeLists.txt
   passes before -T. Hook sections are placed by --section-start flags, not here. */
INCLUDE free.ld
INCLUDE {{.Name}}.ld

SECTIONS
{
  .text : { *(.text*) *(.rodata*) } > ROM_FREE

  /* Writable variables need free work RAM, which is not known yet. */
  .data : { *(.data*) } > ROM_FREE
  .bss (NOLOAD) : { *(.bss*) *(COMMON) } > ROM_FREE
  ASSERT(SIZEOF(.data) == 0, "RAM placement is not defined yet: no initialised globals")
  ASSERT(SIZEOF(.bss) == 0, "RAM placement is not defined yet: no globals")
}
