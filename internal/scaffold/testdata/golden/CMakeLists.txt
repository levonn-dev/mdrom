cmake_minimum_required(VERSION 3.28)
project(synth C ASM)

if(NOT CMAKE_SYSTEM_PROCESSOR STREQUAL "m68k")
  message(FATAL_ERROR "Configure with 'cmake --preset m68k'.")
endif()

set(CLEAN_ROM "${CMAKE_SOURCE_DIR}/base.md" CACHE FILEPATH "Clean ROM image")
if(NOT EXISTS "${CLEAN_ROM}")
  message(FATAL_ERROR "Clean ROM not found at ${CLEAN_ROM}; put it there or pass -DCLEAN_ROM=<path>.")
endif()
set(CLEAN_ROM_SHA1 0123456789abcdef0123456789abcdef01234567)
set(ROM_SIZE 2097152) # 0x200000
# ROM offsets of every .hook_ section in hooks/hooks.s; mdrom fails the inject if one drifts.
set(HOOKS 000004)

include(cmake/mdrom.cmake)

# mdrom measures the ROM and writes the ROM_FREE memory region the linker script includes.
add_custom_command(
  OUTPUT ${CMAKE_BINARY_DIR}/free.ld
  COMMAND ${MDROM} free "${CLEAN_ROM}" --size ${ROM_SIZE} --ld ${CMAKE_BINARY_DIR}/free.ld
  DEPENDS "${CLEAN_ROM}"
  COMMENT "Measuring free ROM space"
  VERBATIM)
add_custom_target(free_ld DEPENDS ${CMAKE_BINARY_DIR}/free.ld)

add_executable(patch.elf
  src/patch.c
  src/rt/div.c
  src/rt/mul.s
  hooks/hooks.s)
add_dependencies(patch.elf free_ld)
target_include_directories(patch.elf PRIVATE game)
set_target_properties(patch.elf PROPERTIES
  LINK_DEPENDS "${CMAKE_SOURCE_DIR}/link/rom.ld;${CMAKE_SOURCE_DIR}/game/synth.ld;${CMAKE_BINARY_DIR}/free.ld")
# -L before -T: the linker resolves INCLUDE against -L paths seen before the script.
# -Xlinker keeps each path one argument, so a checkout under a path with commas or spaces still links.
# One SHELL: entry per pair stops CMake's option de-duplication from folding the -Xlinker tokens.
target_link_options(patch.elf PRIVATE
  "SHELL:-Xlinker -L -Xlinker \"${CMAKE_BINARY_DIR}\""
  "SHELL:-Xlinker -L -Xlinker \"${CMAKE_SOURCE_DIR}/game\""
  "SHELL:-Xlinker -T -Xlinker \"${CMAKE_SOURCE_DIR}/link/rom.ld\""
  "SHELL:-Xlinker -Map -Xlinker \"${CMAKE_BINARY_DIR}/patch.map\"")
foreach(hook IN LISTS HOOKS)
  target_link_options(patch.elf PRIVATE -Wl,--section-start=.hook_${hook}=0x${hook})
endforeach()
add_custom_command(TARGET patch.elf POST_BUILD
  COMMAND ${CMAKE_SOURCE_DIR}/scripts/check-68000.sh $<TARGET_FILE:patch.elf>
  COMMAND ${CMAKE_SOURCE_DIR}/scripts/check-hooks.sh $<TARGET_FILE:patch.elf> ${HOOKS}
  VERBATIM)

# Host-side check of the division helpers; scripts/check-rt.sh holds the compile line.
add_custom_target(check-rt ALL
  COMMAND ${CMAKE_SOURCE_DIR}/scripts/check-rt.sh ${CMAKE_BINARY_DIR}/host
  COMMENT "Checking division helpers against native arithmetic"
  VERBATIM)

# Inject the ELF into the clean ROM, then write the BPS patch.
add_custom_command(
  OUTPUT ${CMAKE_BINARY_DIR}/patched.md
  COMMAND ${MDROM} inject --base "${CLEAN_ROM}" --elf $<TARGET_FILE:patch.elf>
          --out ${CMAKE_BINARY_DIR}/patched.md --size ${ROM_SIZE} --expect-sha1 ${CLEAN_ROM_SHA1}
  DEPENDS patch.elf "${CLEAN_ROM}"
  COMMENT "Injecting patch.elf into the ROM"
  VERBATIM)
add_custom_command(
  OUTPUT ${CMAKE_BINARY_DIR}/patch.bps
  COMMAND ${MDROM} bps create --base "${CLEAN_ROM}" --target ${CMAKE_BINARY_DIR}/patched.md
          --out ${CMAKE_BINARY_DIR}/patch.bps
  DEPENDS ${CMAKE_BINARY_DIR}/patched.md
  COMMENT "Writing patch.bps"
  VERBATIM)
add_custom_target(rom ALL DEPENDS ${CMAKE_BINARY_DIR}/patch.bps)
