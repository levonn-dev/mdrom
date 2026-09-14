/* 32-bit division helpers replacing libgcc, whose prebuilt copy uses the 68020-only bsr.l.
 * Division by zero yields 0; INT32_MIN / -1 wraps to INT32_MIN, as libgcc does. */
#include <stdint.h>

static uint32_t udivmod(uint32_t n, uint32_t d, uint32_t *rem)
{
    if (n <= 0xFFFFu && d <= 0xFFFFu) {
        *rem = (uint16_t)n % (uint16_t)d;
        return (uint16_t)n / (uint16_t)d;
    }
    uint32_t q = 0, r = 0;
    for (int i = 31; i >= 0; i--) {
        r = (r << 1) | ((n >> i) & 1u);
        if (r >= d) {
            r -= d;
            q |= (uint32_t)1 << i;
        }
    }
    *rem = r;
    return q;
}

uint32_t __udivsi3(uint32_t n, uint32_t d)
{
    uint32_t r;
    if (d == 0) return 0;
    return udivmod(n, d, &r);
}

uint32_t __umodsi3(uint32_t n, uint32_t d)
{
    uint32_t r;
    if (d == 0) return 0;
    udivmod(n, d, &r);
    return r;
}

int32_t __divsi3(int32_t n, int32_t d)
{
    uint32_t r;
    if (d == 0) return 0;
    uint32_t un = n < 0 ? 0u - (uint32_t)n : (uint32_t)n;
    uint32_t ud = d < 0 ? 0u - (uint32_t)d : (uint32_t)d;
    uint32_t q = udivmod(un, ud, &r);
    return (n < 0) != (d < 0) ? (int32_t)(0u - q) : (int32_t)q;
}

int32_t __modsi3(int32_t n, int32_t d)
{
    uint32_t r;
    if (d == 0) return 0;
    uint32_t un = n < 0 ? 0u - (uint32_t)n : (uint32_t)n;
    uint32_t ud = d < 0 ? 0u - (uint32_t)d : (uint32_t)d;
    udivmod(un, ud, &r);
    return n < 0 ? (int32_t)(0u - r) : (int32_t)r;
}
