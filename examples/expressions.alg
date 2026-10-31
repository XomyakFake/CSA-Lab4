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

func probe(auto label, auto value) {
    out(label); // по выводу видно, какие операнды реально вычислялись
    return value;
}

func show(auto value) {
    out('=');
    print_int(value);
    out('\n');
}

func squares(auto n) {
    auto a[8]; // локальный массив лежит в кадре стека
    for (auto i = 0; i < n; i = i + 1) {
        a[i] = i * i;
    }
    auto sum = 0;
    for (auto i = 0; i < n; i = i + 1) {
        sum = sum + a[i];
    }
    return sum;
}

// левый операнд 0, правый не вычисляется, напечатается только a
show(probe('a', 0) && probe('b', 1));
// левый операнд не 0, правый не нужен, напечатается только c
show(probe('c', 5) || probe('d', 1));
// тут вычисляются все три
show(probe('e', 1) && probe('f', 0) || probe('g', 2));

auto x = 7;
auto y = 3;
show((x > y) + (x == y) * 10 + (x != y) * 100); // сравнения как числа
show(-x + ~y + !x); // унарные операторы
show(x / y * 100 + x % y); // деление и остаток
show(x & 5 | 8); // побитовые операторы
show(squares(5));

// вложенность больше 7, регистров не хватит и левый операнд уйдёт в стек
show(1 + (2 + (3 + (4 + (5 + (6 + (7 + (8 + (x * y)))))))));
