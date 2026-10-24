package translator

import (
	"fmt"
	"strings"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
)

type node any // вид узла узнаём по типу структуры

type (
	numLit struct { // выражения
		value  int32
		isChar bool
	}
	strLit struct {
		value string
	}
	ident struct {
		name string
		line int
	}
	unary struct {
		op string
		x  node
	}
	binary struct {
		op   string
		l, r node
	}
	assign struct {
		target node // ident или index
		value  node
		line   int
	}
	index struct {
		base, idx node
	}
	call struct {
		name string
		args []node
		line int
	}

	varDecl struct { // операторы
		name string
		init node // может быть nil
		size node // есть только у массива
		line int
	}
	ifStmt struct {
		cond node
		then *block
		els  node // block, ifStmt (else if) или nil
	}
	whileStmt struct {
		cond node
		body *block
	}
	forStmt struct {
		init node // varDecl или выражение, любая часть может быть nil
		cond node
		step node
		body *block
	}
	returnStmt struct {
		value node
		line  int
	}
	block struct {
		stmts []node
	}

	constDecl struct { // объявления верхнего уровня
		name  string
		value node
		line  int
	}
	funcDecl struct {
		name   string
		params []string
		body   *block
		line   int
	}
	program struct {
		items []node
	}
)

func dumpAST(p *program) string {
	var sb strings.Builder
	dump(&sb, p, 0)
	return sb.String()
}

func dump(sb *strings.Builder, n node, depth int) {
	line := func(format string, args ...any) {
		sb.WriteString(strings.Repeat("  ", depth))
		fmt.Fprintf(sb, format, args...)
		sb.WriteByte('\n')
	}
	children := func(nodes ...node) {
		for _, c := range nodes {
			dump(sb, c, depth+1)
		}
	}
	switch n := n.(type) {
	case nil:
		line("Empty")
	case *program:
		line("Program")
		children(n.items...)
	case *constDecl:
		line("Const %s", n.name)
		children(n.value)
	case *funcDecl:
		line("Func %s(%s)", n.name, strings.Join(n.params, ", "))
		children(n.body.stmts...)
	case *varDecl:
		switch {
		case n.size != nil:
			line("Array %s", n.name)
			children(n.size)
		case n.init != nil:
			line("Var %s", n.name)
			children(n.init)
		default:
			line("Var %s", n.name)
		}
	case *ifStmt:
		line("If")
		children(n.cond, n.then)
		if n.els != nil {
			children(n.els)
		}
	case *whileStmt:
		line("While")
		children(n.cond, n.body)
	case *forStmt:
		line("For")
		children(n.init, n.cond, n.step, n.body)
	case *returnStmt:
		line("Return")
		if n.value != nil {
			children(n.value)
		}
	case *block:
		line("Block")
		children(n.stmts...)
	case *numLit:
		if n.isChar {
			line("Char %s", isa.CharRepr(n.value))
		} else {
			line("Num %d", n.value)
		}
	case *strLit:
		line("Str %q", n.value)
	case *ident:
		line("Ident %s", n.name)
	case *unary:
		line("Unary %s", n.op)
		children(n.x)
	case *binary:
		line("Binary %s", n.op)
		children(n.l, n.r)
	case *assign:
		line("Assign")
		children(n.target, n.value)
	case *index:
		line("Index")
		children(n.base, n.idx)
	case *call:
		line("Call %s", n.name)
		children(n.args...)
	default:
		line("?%T", n)
	}
}
