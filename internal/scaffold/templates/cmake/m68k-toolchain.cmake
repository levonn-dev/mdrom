# Bare-metal 68000 toolchain using the Ubuntu m68k-linux-gnu GCC.
set(CMAKE_SYSTEM_NAME Generic)
set(CMAKE_SYSTEM_PROCESSOR m68k)
set(CMAKE_C_COMPILER m68k-linux-gnu-gcc)
set(CMAKE_ASM_COMPILER m68k-linux-gnu-gcc)
set(CMAKE_TRY_COMPILE_TARGET_TYPE STATIC_LIBRARY)

# The compiler defaults to the 68020; -m68000 is mandatory. -g never changes ROM bytes.
set(CMAKE_C_FLAGS_INIT "-m68000 -ffreestanding -nostdlib -fno-pic -fomit-frame-pointer -Os -Wall -Wextra -g")
set(CMAKE_ASM_FLAGS_INIT "-m68000 -g")
# No libgcc: its prebuilt copy targets the 68020. src/rt provides the helpers instead.
set(CMAKE_EXE_LINKER_FLAGS_INIT "-m68000 -nostdlib -static -Wl,--build-id=none -Wl,-z,noexecstack -Wl,--entry=0")

# Build types must not alter code generation; the flags above are the whole story.
foreach(cfg DEBUG RELEASE RELWITHDEBINFO MINSIZEREL)
  set(CMAKE_C_FLAGS_${cfg}_INIT "")
  set(CMAKE_ASM_FLAGS_${cfg}_INIT "")
  set(CMAKE_EXE_LINKER_FLAGS_${cfg}_INIT "")
endforeach()
