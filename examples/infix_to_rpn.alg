const BUF = 64;
auto line[BUF]; // входная строка
auto res[BUF * 2]; // результат как C-строка
auto ops[BUF]; // стек операторов
auto len = 0; // длина результата
auto top = 0; // число операторов в стеке

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
            return -1; // строка не влезла в буфер
        }
        buf[i] = c;
        i = i + 1;
        c = in();
    }
    buf[i] = '\0';
    return i;
}

func priority(auto op) {
    if (op == '*' || op == '/') {
        return 2;
    }
    return 1;
}

func emit(auto c) {
    res[len] = c;
    len = len + 1;
}

func pop_op() {
    top = top - 1;
    emit(ops[top]);
    emit(' ');
}

func convert() { // сортировочная станция, при ошибке в выражении вернёт -1
    auto i = 0;
    while (line[i] != '\0') {
        auto c = line[i];
        if (c == ' ') {
            i = i + 1;
        } else if (c >= '0' && c <= '9') {
            while (line[i] >= '0' && line[i] <= '9') {
                emit(line[i]);
                i = i + 1;
            }
            emit(' ');
        } else if (c == '+' || c == '-' || c == '*' || c == '/') {
            // выталкиваем операторы не слабее текущего, так получается левая ассоциативность
            while (top > 0 && ops[top - 1] != '(' && priority(ops[top - 1]) >= priority(c)) {
                pop_op();
            }
            ops[top] = c;
            top = top + 1;
            i = i + 1;
        } else if (c == '(') {
            ops[top] = c;
            top = top + 1;
            i = i + 1;
        } else if (c == ')') {
            while (top > 0 && ops[top - 1] != '(') {
                pop_op();
            }
            if (top == 0) {
                return -1; // лишняя закрывающая скобка
            }
            top = top - 1;
            i = i + 1;
        } else {
            return -1; // неизвестный символ
        }
    }
    while (top > 0) {
        if (ops[top - 1] == '(') {
            return -1; // осталась незакрытая скобка
        }
        pop_op();
    }
    if (len > 0) {
        len = len - 1; // убрать пробел после последней лексемы
    }
    res[len] = '\0';
    return 0;
}

if (read_line(line) == -1 || convert() == -1) {
    print_str("-1");
} else {
    print_str(res);
}
out('\n');
