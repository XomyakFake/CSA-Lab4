const MAX = 64;
auto x[MAX];
auto y[MAX];

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

func filter(auto src, auto dst, auto n) {
    for (auto i = 0; i < n; i = i + 1) {
        auto acc = 3 * src[i]; // Y[i] = 3*X[i] + 2*X[i-1] + X[i-2], за краем массива нули
        if (i >= 1) {
            acc = acc + 2 * src[i - 1];
        }
        if (i >= 2) {
            acc = acc + src[i - 2];
        }
        dst[i] = acc;
    }
}

auto n = read_int(); // сначала N, потом N чисел
if (n > MAX) {
    n = MAX;
}
for (auto i = 0; i < n; i = i + 1) {
    x[i] = read_int();
}

filter(x, y, n); // соседние X читаются по несколько раз и почти всегда уже лежат в кеше

for (auto i = 0; i < n; i = i + 1) {
    if (i > 0) {
        out(' ');
    }
    print_int(y[i]);
}
out('\n');
