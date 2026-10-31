func print_str(auto s) {
    auto i = 0;
    while (s[i] != '\0') { // печатаем до нуля в конце строки
        out(s[i]);
        i = i + 1;
    }
}

print_str("Hello, world!\n");
