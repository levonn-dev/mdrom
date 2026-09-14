#include <stdint.h>
#include "{{.Name}}.h"

/* Runs first at reset, before the game's own entry point (see hooks/hooks.s).
 * This placeholder exercises the multiply, divide, and modulus helpers in src/rt
 * and returns 356872 (0x57208), readable from d0 with a breakpoint on patch_ret.
 * Replace it with the patch's startup work. volatile stops build-time folding. */
uint32_t patch_init(void)
{
    volatile uint32_t a = 123456789u, b = 1000u, e = 3u;
    volatile int32_t c = -100000, d = 7;
    return (a / b) * e + (uint32_t)(c / d) + (a % b);
}
