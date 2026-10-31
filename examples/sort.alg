const MAX = 32;
auto nums[MAX];

func read_int() {
    auto c = in();
    while (c == ' ' || c == '\n') { // пропускаем пробелы перед числом
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
        print_int(n / 10); // сначала старшие цифры
    }
    out('0' + n % 10);
}

func sort(auto a, auto n) { // сортировка вставками
    for (auto i = 1; i < n; i = i + 1) {
        auto key = a[i];
        auto j = i - 1;
        while (j >= 0 && a[j] > key) {
            a[j + 1] = a[j];
            j = j - 1;
        }
        a[j + 1] = key;
    }
}

auto count = 0;
auto x = read_int();
while (x != 0 && count < MAX) { // список кончается нулём, как C-строка
    nums[count] = x;
    count = count + 1;
    x = read_int();
}

sort(nums, count);

for (auto i = 0; i < count; i = i + 1) {
    if (i > 0) {
        out(' ');
    }
    print_int(nums[i]);
}
out('\n');
