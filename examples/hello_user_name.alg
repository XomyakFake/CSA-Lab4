const BUF = 32;
auto name[BUF];

func print_str(auto s) {
    auto i = 0;
    while (s[i] != '\0') {
        out(s[i]);
        i = i + 1;
    }
}

func read_line(auto buf) {
    auto i = 0;
    auto c = in();
    while (c != '\n') {
        if (i >= BUF - 1) {
            return -1; // имя не влезло в буфер
        }
        buf[i] = c;
        i = i + 1;
        c = in();
    }
    buf[i] = '\0';
    return i;
}

print_str("What is your name?\n");
if (read_line(name) == -1) {
    print_str("name too long\n");
} else {
    print_str("Hello, ");
    print_str(name);
    print_str("!\n");
}
