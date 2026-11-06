package translator

import (
	"strings"
	"testing"
)

func mnemonics(t *testing.T, src string) string {
	t.Helper()
	res, err := Translate(src)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(res.CodeListing), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, ";") { // строка с именем функции
			continue
		}
		parts := strings.SplitN(line, " - ", 3)
		out = append(out, parts[2]) // от строки листинга нужна только мнемоника
	}
	return strings.Join(out, "\n")
}

func TestExpressionMapping(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string // эти команды должны идти в коде подряд
	}{
		// примеры как в docs/memory-isa.md
		{
			"leaves are used as operands",
			"auto x; func f() { auto a = 1; auto b = 2; x = a + 5 * b; }",
			[]string{"mov R1, [FP-4]", "mov R2, #5", "mul R2, [FP-8]", "add R1, R2", "mov [0x0088], R1"},
		},
		{
			"x = x + 1 is one instruction on memory",
			"func f(auto n) { auto steps = 0; steps = steps + 1; n = n / 2; }",
			[]string{"add [FP-4], #1", "div [FP+8], #2"},
		},
		{
			"comparison in a condition",
			"func f(auto n) { if (n <= 0) { return -1; } return n; }",
			[]string{"cmp [FP+8], #0", "jgt #0x0034", "mov R0, #-1"},
		},
		{
			"comparison as a value",
			"auto a; auto b; auto x; x = a < b;",
			[]string{"cmp [0x0088], [0x008C]", "slt R1", "mov [0x0090], R1"},
		},
		{
			"global array element by index",
			"auto i; auto a[4]; a[i] = a[2] + 1;",
			[]string{"mov R1, [0x0094]", "add R1, #1", "mov R2, [0x0088]", "mul R2, #4", "mov [R2+140], R1"},
		},
		{
			"arguments are pushed by one push",
			"func f(auto a, auto b, auto c) { return a; } auto x = 3; f(x, 5, x + 1);",
			[]string{"mov R1, [0x0088]", "add R1, #1", "push [0x0088], #5, R1", "call #0x0048", "add SP, #12"},
		},
		{
			"live registers are saved around a call",
			"func f() { return 1; } auto x = 2; x = x * 3 + f();",
			[]string{"push R1", "call #0x0048", "pop R1", "mov R2, R0", "add R1, R2"},
		},
		{
			"string literal is passed as an address in data memory",
			`func f(auto s) { return s[0]; } f("hi");`,
			[]string{"push #136", "call #0x0020"},
		},
		{
			"io is ordinary mov with ports",
			"out(in());",
			[]string{"mov [0x0084], [0x0080]"},
		},
		{
			"add64 uses add and adc",
			"auto a[2]; auto b[2]; auto r[2]; add64(r, a, b);",
			[]string{"mov R4, [R2]", "add R4, [R3]", "mov [R1], R4", "mov R4, [R2+4]", "adc R4, [R3+4]", "mov [R1+4], R4"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mnemonics(t, c.src)
			if !strings.Contains(got, strings.Join(c.want, "\n")) {
				t.Errorf("code:\n%s\ndoes not contain:\n%s", got, strings.Join(c.want, "\n"))
			}
		})
	}
}

func TestStringInDataMemory(t *testing.T) {
	res, err := Translate(`auto x; auto a[2]; func f(auto s) { return s; } f("hi");`)
	if err != nil {
		t.Fatal(err)
	}
	// переменные, потом массивы, потом строка по символу на слово и 0 в конце
	want := "0088 - 00000000 - x\n" +
		"008C - 00000000 - a[0]\n" +
		"0090 - 00000000 - a[1]\n" +
		"0094 - 00000068 - 'h'\n" +
		"0098 - 00000069 - 'i'\n" +
		"009C - 00000000 - '\\0'\n"
	if res.DataListing != want {
		t.Errorf("data listing:\n%s\nwant:\n%s", res.DataListing, want)
	}
}

func TestTranslateErrors(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"undeclared variable", "x = 1;", `undeclared name "x"`},
		{"use before declaration", "x = 1; auto x;", `undeclared name "x"`},
		{"redeclaration", "auto x; auto x;", "already declared"},
		{"assignment to constant", "const N = 1; N = 2;", "cannot assign"},
		{"assignment to array", "auto a[2]; a = 1;", "cannot assign"},
		{"assignment to expression", "auto a; a + 1 = 2;", "left side of assignment"},
		{"non-constant array size", "auto n; auto a[n];", "positive constant"},
		{"non-constant constant", "auto n; const N = n;", "not known at translation time"},
		{"return outside function", "return 1;", "return outside"},
		{"undeclared function", "f();", `undeclared function "f"`},
		{"wrong argument count", "func f(auto a) { return a; } f(1, 2);", "takes 1 arguments"},
		{"builtin name", "auto in;", "built-in"},
		{"missing semicolon", "auto x = 1", `expected ";"`},
		{"unterminated string", `out("abc);`, "unterminated"},
		{"bad character", "auto x = 1 @ 2;", "unexpected character"},
		{"number too large", "auto x = 4294967296;", "bad number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Translate(c.src)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got error %v, want one containing %q", err, c.want)
			}
		})
	}
}

func TestASTDump(t *testing.T) {
	res, err := Translate("const N = 2;\nauto a[N];\nfor (auto i = 0; i < N; i = i + 1) {\n    a[i] = -i;\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	want := `Program
  Const N
    Num 2
  Array a
    Ident N
  For
    Var i
      Num 0
    Binary <
      Ident i
      Ident N
    Assign
      Ident i
      Binary +
        Ident i
        Num 1
    Block
      Assign
        Index
          Ident a
          Ident i
        Unary -
          Ident i
`
	if res.AST != want {
		t.Errorf("AST:\n%s\nwant:\n%s", res.AST, want)
	}
}
