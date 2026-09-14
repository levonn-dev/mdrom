/* Checks the division helpers in src/rt/div.c against native arithmetic on the host.
 * Compiled with the host gcc by the check-rt target. */
#include <stdint.h>
#include <stdio.h>

uint32_t __udivsi3(uint32_t n, uint32_t d);
uint32_t __umodsi3(uint32_t n, uint32_t d);
int32_t __divsi3(int32_t n, int32_t d);
int32_t __modsi3(int32_t n, int32_t d);

static const uint32_t samples[] = {
    0, 1, 2, 3, 7, 10, 255, 256, 1000, 65535, 65536, 123456789,
    0x7FFFFFFFu, 0x80000000u, 0xFFFFFFFEu, 0xFFFFFFFFu,
};

int main(void)
{
    int failures = 0;
    size_t n = sizeof samples / sizeof samples[0];
    for (size_t i = 0; i < n; i++) {
        for (size_t j = 0; j < n; j++) {
            uint32_t a = samples[i], b = samples[j];
            int32_t sa = (int32_t)a, sb = (int32_t)b;
            if (b == 0) {
                if (__udivsi3(a, b) != 0 || __umodsi3(a, b) != 0 || __divsi3(sa, sb) != 0 || __modsi3(sa, sb) != 0) {
                    printf("divide by zero must yield 0 (a=%u)\n", a);
                    failures++;
                }
                continue;
            }
            if (__udivsi3(a, b) != a / b) { printf("udiv %u/%u\n", a, b); failures++; }
            if (__umodsi3(a, b) != a % b) { printf("umod %u%%%u\n", a, b); failures++; }
            if (sa == INT32_MIN && sb == -1) {
                if (__divsi3(sa, sb) != INT32_MIN || __modsi3(sa, sb) != 0) { printf("INT32_MIN/-1 must wrap\n"); failures++; }
                continue;
            }
            if (__divsi3(sa, sb) != sa / sb) { printf("div %d/%d\n", sa, sb); failures++; }
            if (__modsi3(sa, sb) != sa % sb) { printf("mod %d%%%d\n", sa, sb); failures++; }
        }
    }
    if (failures) {
        printf("rt_test: %d failures\n", failures);
        return 1;
    }
    printf("rt_test: %zu pairs ok\n", n * n);
    return 0;
}
