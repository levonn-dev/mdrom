/* Fixture: initialised data placed in RAM with its load image in ROM, plus a bss variable. */
unsigned short at_table[4] = {1, 2, 3, 4};
unsigned short at_counter;
unsigned short at_sum(void) { return (unsigned short)(at_table[0] + at_table[3] + at_counter); }
