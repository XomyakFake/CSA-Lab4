auto a[2]; // a[0] младшее слово, a[1] старшее
auto b[2];
auto r[2];

func hex_value(auto c) {
    if (c >= '0' && c <= '9') {
        return c - '0';
    }
    if (c >= 'a' && c <= 'f') {
        return c - 'a' + 10;
    }
    return c - 'A' + 10;
}

func read_word() {
    auto w = 0;
    for (auto i = 0; i < 8; i = i + 1) {
        w = w * 16 + hex_value(in()); // может уйти в минус, но биты получатся верные
    }
    return w;
}

func read64(auto x) {
    x[1] = read_word(); // старшее слово записано первым
    x[0] = read_word();
    in(); // разделитель после числа
}

func print_digit(auto d) {
    if (d < 10) {
        out('0' + d);
    } else {
        out('A' + d - 10);
    }
}

func print_word(auto w) {
    // слово знаковое, а деление округляет к нулю, поэтому старший бит обрабатываем отдельно
    auto top = 0;
    if (w < 0) {
        top = 8;
        w = w & 0x7FFFFFFF;
    }
    auto div = 0x10000000;
    print_digit(top + w / div);
    for (auto i = 0; i < 7; i = i + 1) {
        w = w % div;
        div = div / 16;
        print_digit(w / div);
    }
}

func print64(auto x) {
    print_word(x[1]);
    print_word(x[0]);
    out('\n');
}

read64(a);
read64(b);
print64(add64(r, a, b)); // внутри add и adc, перенос идёт через флаг C
print64(sub64(r, a, b));
print64(sub64(r, b, a));
