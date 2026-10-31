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

func factorial(auto n) {
    if (n <= 1) {
        return 1;
    }
    return n * factorial(n - 1);
}

func fib(auto n) {
    if (n < 2) {
        return n;
    }
    // пока считается fib(n - 2), результат fib(n - 1) лежит в регистре и сохраняется в стеке
    return fib(n - 1) + fib(n - 2);
}

func gcd(auto a, auto b) {
    if (b == 0) {
        return a;
    }
    return gcd(b, a % b);
}

func max3(auto a, auto b, auto c) {
    auto m = a;
    if (b > m) {
        m = b;
    }
    if (c > m) {
        m = c;
    }
    return m;
}

print_int(factorial(6));
out(' ');
print_int(fib(10));
out(' ');
print_int(gcd(84, 36));
out(' ');
print_int(max3(7, factorial(4), fib(8))); // три аргумента уходят одной командой push
out('\n');
