func read_int() {
    auto c = in();
    while (c == ' ' || c == '\n') {
        c = in();
    }
    auto sign = 1;
    if (c == '-') {
        sign = -1;
        c = in();
    }
    auto n = 0;
    while (c >= '0' && c <= '9') {
        n = n * 10 + (c - '0');
        c = in();
    }
    return n * sign;
}

func print_int(auto n) {
    if (n < 0) {
        out('-');
        n = -n;
    }
    if (n >= 10) {
        print_int(n / 10);
    }
    out('0' + n % 10);
}

func collatz_length(auto n) {
    if (n <= 0) { // по условию для таких n ответ -1
        return -1;
    }
    auto steps = 0;
    while (n != 1) {
        if (n % 2 == 0) {
            n = n / 2;
        } else {
            n = 3 * n + 1;
        }
        steps = steps + 1;
    }
    return steps;
}

print_int(collatz_length(read_int()));
out('\n');
