/* Fixture for elfimg tests: one function so .text has known bytes. */
unsigned short poc_value(unsigned short x) { return (unsigned short)(x * 3u + 1u); }
