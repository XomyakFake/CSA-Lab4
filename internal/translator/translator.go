package translator

import (
	"errors"
	"strings"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
)

type Result struct {
	AST         string
	Program     isa.Program
	CodeListing string
	DataListing string
}

func (r *Result) Listing() string {
	return "; instruction memory\n" + r.CodeListing + "\n; data memory\n" + r.DataListing
}

func Translate(src string) (res *Result, err error) {
	defer func() {
		// ошибки трансляции прилетают через panic, чужие panic отдаём дальше
		if r := recover(); r != nil {
			var te translateError
			if e, ok := r.(error); ok && errors.As(e, &te) {
				res, err = nil, te
				return
			}
			panic(r)
		}
	}()
	prog := parse(src)
	g := newGenerator()
	g.genProgram(prog)
	res = g.link()
	res.AST = dumpAST(prog)
	return res, nil
}

func (g *generator) link() *Result {
	addrs := map[string]int32{}

	// сначала код верхнего уровня с адреса 0, за ним функции. Длина команды
	// не зависит от адресов меток, поэтому хватает одного прохода
	pc := int32(0)
	starts := map[bool][]int32{}
	for _, inFuncs := range []bool{false, true} {
		buf := g.main
		if inFuncs {
			buf = g.funcs
		}
		for _, in := range buf {
			starts[inFuncs] = append(starts[inFuncs], pc)
			pc += 4 * int32(isa.Instr{Op: in.op, Operands: plain(in.ops, nil)}.Size())
		}
		starts[inFuncs] = append(starts[inFuncs], pc) // метка после последней команды
	}
	funcNames := map[int]string{}
	for name, pos := range g.codeLabels {
		addrs[name] = starts[pos.inFuncs][pos.idx]
		if fn, ok := strings.CutPrefix(name, "func "); ok {
			funcNames[int(addrs[name])] = fn
		}
	}

	// статика с 0x88, сначала переменные, потом массивы и строки
	data := make([]uint32, isa.StaticBase/4)
	notes := map[int]string{}
	for _, group := range [][]dataItem{g.vars, g.arrays, g.strs} {
		for _, item := range group {
			addrs[item.label] = int32(4 * len(data))
			for i, w := range item.words {
				notes[4*len(data)] = item.notes[i]
				data = append(data, w)
			}
		}
	}

	var code []uint32
	for _, buf := range [][]instr{g.main, g.funcs} {
		for _, in := range buf {
			code = append(code, isa.Instr{Op: in.op, Operands: plain(in.ops, addrs)}.Encode()...)
		}
	}
	if len(code) > isa.MemSize/4 || len(data) > isa.MemSize/4 {
		failf(0, "program does not fit into memory")
	}
	return &Result{
		Program:     isa.Program{Code: code, Data: data},
		CodeListing: isa.CodeListing(code, funcNames),
		DataListing: isa.DataListing(data, notes),
	}
}

func plain(ops []operand, addrs map[string]int32) []isa.Operand {
	out := make([]isa.Operand, len(ops))
	for i, o := range ops {
		out[i] = o.Operand
		if o.label != "" && addrs != nil { // addrs == nil когда нужна только длина команды
			out[i].Value += addrs[o.label] // так получается и [R2+140], и [0x0090] для a[2]
		}
	}
	return out
}
