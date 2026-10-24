package translator

import (
	"fmt"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
)

type operand struct {
	isa.Operand
	label string // адрес метки прибавится к Value при компоновке
}

type instr struct {
	op  isa.Opcode
	ops []operand
}

type dataItem struct {
	label string
	words []uint32
	notes []string // подписи слов для листинга
}

type symKind int

const (
	symConst     symKind = iota // в value само значение
	symGlobal                   // в label метка в статике
	symGlobalArr                // тоже метка в статике
	symLocal                    // локальная или параметр, в value смещение от FP
	symLocalArr                 // в value смещение первого элемента от FP
)

type symbol struct {
	kind  symKind
	value int32
	label string
}

type labelPos struct {
	inFuncs bool // метка в коде функций, а не в коде верхнего уровня
	idx     int
}

var builtins = map[string]int{"in": 0, "out": 1, "add64": 3, "sub64": 3}

var arithOps = map[string]isa.Opcode{
	"+": isa.OpAdd, "-": isa.OpSub, "*": isa.OpMul, "/": isa.OpDiv,
	"%": isa.OpRem, "&": isa.OpAnd, "|": isa.OpOr,
}

var compareOps = map[string][3]isa.Opcode{ // установка, переход если да, переход если нет
	"==": {isa.OpSeq, isa.OpJeq, isa.OpJne},
	"!=": {isa.OpSne, isa.OpJne, isa.OpJeq},
	"<":  {isa.OpSlt, isa.OpJlt, isa.OpJge},
	"<=": {isa.OpSle, isa.OpJle, isa.OpJgt},
	">":  {isa.OpSgt, isa.OpJgt, isa.OpJle},
	">=": {isa.OpSge, isa.OpJge, isa.OpJlt},
}

type generator struct {
	main, funcs []instr // код верхнего уровня и код функций
	inFunc      bool    // сейчас генерируем тело функции
	codeLabels  map[string]labelPos
	labelCount  int

	vars, arrays, strs []dataItem
	strLabels          map[*strLit]string

	scopes    []map[string]symbol
	funcArity map[string]int

	used      uint8  // сколько регистров пула R1..R7 занято
	frame     int32  // сколько слов кадра уже занято локальными
	exitLabel string // метка эпилога текущей функции
}

func newGenerator() *generator {
	return &generator{
		codeLabels: map[string]labelPos{},
		strLabels:  map[*strLit]string{},
		scopes:     []map[string]symbol{{}},
		funcArity:  map[string]int{},
	}
}

func (g *generator) emit(op isa.Opcode, ops ...operand) {
	if g.inFunc {
		g.funcs = append(g.funcs, instr{op, ops})
	} else {
		g.main = append(g.main, instr{op, ops})
	}
}

func reg(r uint8) operand        { return operand{Operand: isa.Reg(r)} }
func imm(v int32) operand        { return operand{Operand: isa.Imm(v)} }
func labelImm(l string) operand  { return operand{Operand: isa.Imm(0), label: l} }
func fpOff(off int32) operand    { return operand{Operand: isa.Off(isa.RegFP, off)} }
func absLabel(l string) operand  { return operand{Operand: isa.Abs(0), label: l} }
func (o operand) isMemory() bool { return o.IsMem() }

func (g *generator) newLabel() string {
	g.labelCount++
	return fmt.Sprintf(".L%d", g.labelCount)
}

func (g *generator) placeLabel(name string) {
	if g.inFunc {
		g.codeLabels[name] = labelPos{true, len(g.funcs)}
	} else {
		g.codeLabels[name] = labelPos{false, len(g.main)}
	}
}

func (g *generator) alloc(line int) uint8 {
	if g.used == isa.PoolSize {
		failf(line, "expression is too complex: out of temporary registers")
	}
	g.used++
	return g.used
}

func (g *generator) free(r uint8) {
	if r != g.used { // пул работает как стек, освобождать можно только верхний
		panic(fmt.Sprintf("register pool is not used as a stack: free R%d, top R%d", r, g.used))
	}
	g.used--
}

func (g *generator) pushScope() { g.scopes = append(g.scopes, map[string]symbol{}) }
func (g *generator) popScope()  { g.scopes = g.scopes[:len(g.scopes)-1] }

func (g *generator) lookup(name string) (symbol, bool) {
	for i := len(g.scopes) - 1; i >= 0; i-- {
		if s, ok := g.scopes[i][name]; ok {
			return s, true
		}
	}
	return symbol{}, false
}

func (g *generator) declare(name string, line int, s symbol) {
	if _, ok := builtins[name]; ok {
		failf(line, "%q is a built-in function name", name)
	}
	top := g.scopes[len(g.scopes)-1]
	if _, ok := top[name]; ok {
		failf(line, "%q is already declared in this block", name)
	}
	top[name] = s
}

func boolToInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func (g *generator) constValue(e node) (int32, bool) {
	switch e := e.(type) {
	case *numLit:
		return e.value, true
	case *ident:
		if s, ok := g.lookup(e.name); ok && s.kind == symConst {
			return s.value, true
		}
	case *unary:
		x, ok := g.constValue(e.x)
		if !ok {
			return 0, false
		}
		switch e.op {
		case "-":
			return -x, true
		case "~":
			return ^x, true
		case "!":
			return boolToInt(x == 0), true
		}
	case *binary:
		l, ok1 := g.constValue(e.l)
		r, ok2 := g.constValue(e.r)
		if !ok1 || !ok2 {
			return 0, false
		}
		return foldBinary(e.op, l, r)
	}
	return 0, false
}

func foldBinary(op string, l, r int32) (int32, bool) {
	switch op {
	case "+":
		return l + r, true
	case "-":
		return l - r, true
	case "*":
		return l * r, true
	case "/", "%":
		if r == 0 || (l == -1<<31 && r == -1) {
			return 0, false // такие случаи оставляем процессору
		}
		if op == "/" {
			return l / r, true
		}
		return l % r, true
	case "&":
		return l & r, true
	case "|":
		return l | r, true
	case "==":
		return boolToInt(l == r), true
	case "!=":
		return boolToInt(l != r), true
	case "<":
		return boolToInt(l < r), true
	case "<=":
		return boolToInt(l <= r), true
	case ">":
		return boolToInt(l > r), true
	case ">=":
		return boolToInt(l >= r), true
	case "&&":
		return boolToInt(l != 0 && r != 0), true
	case "||":
		return boolToInt(l != 0 || r != 0), true
	}
	return 0, false
}

func hasSideEffects(e node) bool {
	switch e := e.(type) {
	case *assign, *call:
		return true // присваивание или вызов, in() тоже считается
	case *unary:
		return hasSideEffects(e.x)
	case *binary:
		return hasSideEffects(e.l) || hasSideEffects(e.r)
	case *index:
		return hasSideEffects(e.base) || hasSideEffects(e.idx)
	}
	return false
}

func (g *generator) stringLabel(s *strLit) string {
	if label, ok := g.strLabels[s]; ok {
		return label
	}
	item := dataItem{label: fmt.Sprintf("str%d", len(g.strs))}
	for i := 0; i < len(s.value); i++ {
		item.words = append(item.words, uint32(s.value[i]))
		item.notes = append(item.notes, isa.CharRepr(int32(s.value[i])))
	}
	item.words = append(item.words, 0)
	item.notes = append(item.notes, isa.CharRepr(0))
	g.strs = append(g.strs, item)
	g.strLabels[s] = item.label
	return item.label
}

func (g *generator) leaf(e node) (operand, bool) {
	// лист подставляется прямо в команду и не занимает регистр
	if v, ok := g.constValue(e); ok {
		return imm(v), true
	}
	switch e := e.(type) {
	case *strLit:
		return labelImm(g.stringLabel(e)), true
	case *ident:
		s := g.mustLookup(e)
		switch s.kind {
		case symGlobal:
			return absLabel(s.label), true
		case symGlobalArr:
			return labelImm(s.label), true
		case symLocal:
			return fpOff(s.value), true
		default: // константа уже обработана выше, локальный массив не лист
		}
	case *call:
		if e.name == "in" {
			g.checkArgs(e)
			return operand{Operand: isa.Abs(isa.PortIn)}, true // in() это просто [0x0080]
		}
	case *index:
		c, ok := g.constValue(e.idx)
		base, isIdent := e.base.(*ident)
		if !ok || !isIdent {
			return operand{}, false
		}
		s := g.mustLookup(base)
		switch s.kind {
		case symGlobalArr:
			return operand{Operand: isa.Abs(4 * c), label: s.label}, true
		case symLocalArr:
			return fpOff(s.value + 4*c), true
		default: // база не массив, тогда адрес считаем в регистре
		}
	}
	return operand{}, false
}

func (g *generator) mustLookup(id *ident) symbol {
	s, ok := g.lookup(id.name)
	if !ok {
		failf(id.line, "undeclared name %q", id.name)
	}
	return s
}

func (g *generator) checkArgs(c *call) {
	want, ok := builtins[c.name]
	if !ok {
		want, ok = g.funcArity[c.name]
	}
	if !ok {
		failf(c.line, "undeclared function %q", c.name)
	}
	if len(c.args) != want {
		failf(c.line, "function %q takes %d arguments, got %d", c.name, want, len(c.args))
	}
}

func (g *generator) genExpr(e node) uint8 {
	if op, ok := g.leaf(e); ok {
		r := g.alloc(0)
		g.emit(isa.OpMov, reg(r), op)
		return r
	}
	switch e := e.(type) {
	case *ident: // сюда доходит только локальный массив, его адрес FP + смещение
		s := g.mustLookup(e)
		r := g.alloc(e.line)
		g.emit(isa.OpMov, reg(r), reg(isa.RegFP))
		g.emit(isa.OpAdd, reg(r), imm(s.value))
		return r
	case *unary:
		r := g.genExpr(e.x)
		switch e.op {
		case "-":
			g.emit(isa.OpNeg, reg(r))
		case "~":
			g.emit(isa.OpNot, reg(r))
		case "!":
			g.emit(isa.OpCmp, reg(r), imm(0))
			g.emit(isa.OpSeq, reg(r))
		}
		return r
	case *binary:
		return g.genBinary(e)
	case *assign:
		return g.genAssign(e, true)
	case *index:
		op, hasReg := g.indexOperand(e)
		if !hasReg {
			panic("indexOperand without register must be a leaf")
		}
		g.emit(isa.OpMov, reg(g.used), op)
		return g.used
	case *call:
		return g.genCall(e, true)
	}
	panic(fmt.Sprintf("unexpected expression node %T", e))
}

func (g *generator) genBinary(e *binary) uint8 {
	if op, ok := arithOps[e.op]; ok {
		return g.genBinOp(op, e.l, e.r)
	}
	if ops, ok := compareOps[e.op]; ok {
		var r uint8
		if g.bothLeaves(e) {
			g.genCompare(e)
			r = g.alloc(0)
		} else {
			r = g.genBinOp(isa.OpCmp, e.l, e.r)
		}
		g.emit(ops[0], reg(r))
		return r
	}
	// && и || как значение, собираем 1 или 0 через переходы
	r := g.alloc(0)
	falseLabel, endLabel := g.newLabel(), g.newLabel()
	g.genBranch(e, falseLabel, false)
	g.emit(isa.OpMov, reg(r), imm(1))
	g.emit(isa.OpJmp, labelImm(endLabel))
	g.placeLabel(falseLabel)
	g.emit(isa.OpMov, reg(r), imm(0))
	g.placeLabel(endLabel)
	return r
}

func (g *generator) genBinOp(op isa.Opcode, l, r node) uint8 {
	rl := g.genExpr(l) // левый всегда первым, так сохраняется порядок слева направо
	if right, ok := g.leaf(r); ok {
		g.emit(op, reg(rl), right)
		return rl
	}
	if g.used < isa.PoolSize {
		rr := g.genExpr(r)
		g.emit(op, reg(rl), reg(rr))
		g.free(rr)
		return rl
	}
	// регистры кончились, левый уходит в стек и потом достаётся в R0
	g.emit(isa.OpPush, reg(rl))
	g.free(rl)
	rr := g.genExpr(r)
	g.emit(isa.OpPop, reg(0))
	g.emit(op, reg(0), reg(rr))
	if op != isa.OpCmp {
		g.emit(isa.OpMov, reg(rr), reg(0))
	}
	return rr
}

func (g *generator) bothLeaves(e *binary) bool {
	_, ok1 := g.leaf(e.l)
	_, ok2 := g.leaf(e.r)
	return ok1 && ok2
}

func (g *generator) genCompare(e *binary) {
	if g.bothLeaves(e) {
		l, _ := g.leaf(e.l)
		r, _ := g.leaf(e.r)
		g.emit(isa.OpCmp, l, r)
		return
	}
	g.free(g.genBinOp(isa.OpCmp, e.l, e.r)) // после cmp нужны только флаги
}

func (g *generator) indexOperand(e *index) (op operand, hasReg bool) {
	// если понадобился регистр под адрес, он остаётся занятым и его освобождает вызывающий
	if op, ok := g.leaf(e); ok {
		return op, false
	}
	if c, ok := g.constValue(e.idx); ok {
		r := g.genExpr(e.base)
		if c == 0 {
			return operand{Operand: isa.Ind(r)}, true
		}
		return operand{Operand: isa.Off(r, 4*c)}, true
	}
	if base, ok := e.base.(*ident); ok {
		switch s := g.mustLookup(base); s.kind {
		case symGlobalArr: // [Rn + адрес массива], где Rn = 4 * индекс
			r := g.genExpr(e.idx)
			g.emit(isa.OpMul, reg(r), imm(4))
			return operand{Operand: isa.Off(r, 0), label: s.label}, true
		case symLocalArr: // [Rn + смещение массива], где Rn = FP + 4 * индекс
			r := g.genExpr(e.idx)
			g.emit(isa.OpMul, reg(r), imm(4))
			g.emit(isa.OpAdd, reg(r), reg(isa.RegFP))
			return operand{Operand: isa.Off(r, s.value)}, true
		default: // база не массив, разбираем ниже
		}
	}
	if base, ok := g.leaf(e.base); ok && !hasSideEffects(e.base) && !hasSideEffects(e.idx) {
		// база это переменная с адресом, например параметр s в s[i]
		r := g.genExpr(e.idx)
		g.emit(isa.OpMul, reg(r), imm(4))
		g.emit(isa.OpAdd, reg(r), base)
		return operand{Operand: isa.Ind(r)}, true
	}
	// общий случай, адрес = база + индекс * 4
	r := g.genBinOp(isa.OpAdd, e.base, &binary{op: "*", l: e.idx, r: &numLit{value: 4}})
	return operand{Operand: isa.Ind(r)}, true
}

func (g *generator) genAssign(a *assign, want bool) uint8 {
	if target, ok := a.target.(*ident); ok {
		s := g.mustLookup(target)
		if s.kind != symGlobal && s.kind != symLocal {
			failf(a.line, "cannot assign to %q", target.name)
		}
		dst, _ := g.leaf(target)
		return g.genStore(dst, target.name, a.value, want)
	}
	target := a.target.(*index)
	// простое значение пишем одним mov
	if src, ok := g.leaf(a.value); ok && !want && (!hasSideEffects(a.value) || !hasSideEffects(target)) {
		dst, hasReg := g.indexOperand(target)
		g.emit(isa.OpMov, dst, src)
		if hasReg {
			g.free(g.used)
		}
		return 0
	}
	// сначала правая часть, потом адрес левой
	rv := g.genExpr(a.value)
	dst, hasReg := g.indexOperand(target)
	g.emit(isa.OpMov, dst, reg(rv))
	if hasReg {
		g.free(g.used)
	}
	if !want {
		g.free(rv)
	}
	return rv
}

func (g *generator) genStore(dst operand, name string, value node, want bool) uint8 {
	// x = x op y становится одной командой над памятью, например add [FP-4], #1
	if b, ok := value.(*binary); ok && !want {
		op, isArith := arithOps[b.op]
		left, isIdent := b.l.(*ident)
		if isArith && isIdent && left.name == name && !hasSideEffects(b.r) {
			if src, ok := g.leaf(b.r); ok {
				g.emit(op, dst, src)
			} else {
				r := g.genExpr(b.r)
				g.emit(op, dst, reg(r))
				g.free(r)
			}
			return 0
		}
	}
	if src, ok := g.leaf(value); ok && !want {
		g.emit(isa.OpMov, dst, src)
		return 0
	}
	r := g.genExpr(value)
	g.emit(isa.OpMov, dst, reg(r))
	if !want {
		g.free(r)
	}
	return r
}

func (g *generator) genCall(c *call, want bool) uint8 {
	g.checkArgs(c)
	switch c.name {
	case "out":
		return g.genOut(c, want)
	case "add64":
		return g.genWide(c, isa.OpAdd, isa.OpAdc, want)
	case "sub64":
		return g.genWide(c, isa.OpSub, isa.OpSbc, want)
	}

	// листья идут прямо в push, но лист из памяти читается только в момент push,
	// поэтому если дальше есть вызов или присваивание, копируем его в регистр
	saved := g.used
	args := make([]operand, 0, len(c.args))
	for i, arg := range c.args {
		laterEffects := false
		for _, later := range c.args[i+1:] {
			laterEffects = laterEffects || hasSideEffects(later)
		}
		if op, ok := g.leaf(arg); ok && (!op.isMemory() || !laterEffects) {
			args = append(args, op)
		} else {
			args = append(args, reg(g.genExpr(arg)))
		}
	}
	// занятые регистры сохраняет вызывающий (caller-saved)
	var live []operand
	for r := uint8(1); r <= saved; r++ {
		live = append(live, reg(r))
	}
	if len(live) > 0 {
		g.emit(isa.OpPush, live...)
	}
	if len(args) > 0 {
		g.emit(isa.OpPush, args...)
	}
	g.emit(isa.OpCall, labelImm("func "+c.name))
	if len(args) > 0 {
		g.emit(isa.OpAdd, reg(isa.RegSP), imm(int32(4*len(args))))
	}
	g.used = saved
	if len(live) > 0 {
		for i, j := 0, len(live)-1; i < j; i, j = i+1, j-1 { // pop в обратном порядке
			live[i], live[j] = live[j], live[i]
		}
		g.emit(isa.OpPop, live...)
	}
	if !want {
		return 0
	}
	r := g.alloc(c.line)
	g.emit(isa.OpMov, reg(r), reg(0))
	return r
}

func (g *generator) genOut(c *call, want bool) uint8 {
	port := operand{Operand: isa.Abs(isa.PortOut)} // out это обычный mov в порт
	if src, ok := g.leaf(c.args[0]); ok && !want {
		g.emit(isa.OpMov, port, src)
		return 0
	}
	r := g.genExpr(c.args[0])
	g.emit(isa.OpMov, port, reg(r))
	if !want {
		g.free(r)
	}
	return r
}

func (g *generator) genWide(c *call, low, high isa.Opcode, want bool) uint8 {
	// младшие слова через add или sub, старшие через adc или sbc
	// mov флаги не трогает, поэтому перенос в C доживает до второй операции
	res := g.genExpr(c.args[0])
	a := g.genExpr(c.args[1])
	b := g.genExpr(c.args[2])
	tmp := g.alloc(c.line)
	g.emit(isa.OpMov, reg(tmp), operand{Operand: isa.Ind(a)})
	g.emit(low, reg(tmp), operand{Operand: isa.Ind(b)})
	g.emit(isa.OpMov, operand{Operand: isa.Ind(res)}, reg(tmp))
	g.emit(isa.OpMov, reg(tmp), operand{Operand: isa.Off(a, 4)})
	g.emit(high, reg(tmp), operand{Operand: isa.Off(b, 4)})
	g.emit(isa.OpMov, operand{Operand: isa.Off(res, 4)}, reg(tmp))
	g.free(tmp)
	g.free(b)
	g.free(a)
	if !want {
		g.free(res)
	}
	return res
}

func (g *generator) genBranch(e node, label string, jumpIfTrue bool) {
	if v, ok := g.constValue(e); ok { // условие известно заранее, например while (1)
		if (v != 0) == jumpIfTrue {
			g.emit(isa.OpJmp, labelImm(label))
		}
		return
	}
	switch e := e.(type) {
	case *unary:
		if e.op == "!" {
			g.genBranch(e.x, label, !jumpIfTrue)
			return
		}
	case *binary:
		if ops, ok := compareOps[e.op]; ok {
			g.genCompare(e)
			if jumpIfTrue {
				g.emit(ops[1], labelImm(label))
			} else {
				g.emit(ops[2], labelImm(label))
			}
			return
		}
		// ленивые && и ||, правый операнд обходим переходом если ответ уже ясен
		if e.op == "&&" || e.op == "||" {
			if (e.op == "&&") != jumpIfTrue {
				// && при переходе по лжи и || при переходе по истине, каждый операнд прыгает сам
				g.genBranch(e.l, label, jumpIfTrue)
				g.genBranch(e.r, label, jumpIfTrue)
				return
			}
			// иначе левый операнд может сразу обойти правый
			skip := g.newLabel()
			g.genBranch(e.l, skip, !jumpIfTrue)
			g.genBranch(e.r, label, jumpIfTrue)
			g.placeLabel(skip)
			return
		}
	}
	// любое другое значение сравниваем с нулём
	if op, ok := g.leaf(e); ok {
		g.emit(isa.OpCmp, op, imm(0))
	} else {
		r := g.genExpr(e)
		g.emit(isa.OpCmp, reg(r), imm(0))
		g.free(r)
	}
	if jumpIfTrue {
		g.emit(isa.OpJne, labelImm(label))
	} else {
		g.emit(isa.OpJeq, labelImm(label))
	}
}

func (g *generator) genBlock(b *block) {
	g.pushScope()
	for _, s := range b.stmts {
		g.genStmt(s)
	}
	g.popScope()
}

func (g *generator) genStmt(s node) {
	switch s := s.(type) {
	case *varDecl:
		g.genVarDecl(s)
	case *ifStmt:
		elseLabel := g.newLabel()
		g.genBranch(s.cond, elseLabel, false)
		g.genBlock(s.then)
		if s.els == nil {
			g.placeLabel(elseLabel)
			return
		}
		endLabel := g.newLabel()
		g.emit(isa.OpJmp, labelImm(endLabel))
		g.placeLabel(elseLabel)
		g.genStmt(s.els)
		g.placeLabel(endLabel)
	case *whileStmt:
		g.genLoop(s.cond, nil, s.body)
	case *forStmt:
		g.pushScope() // переменная из for видна только внутри цикла
		if s.init != nil {
			g.genStmt(s.init)
		}
		g.genLoop(s.cond, s.step, s.body)
		g.popScope()
	case *returnStmt:
		g.genReturn(s, true)
	case *block:
		g.genBlock(s)
	default:
		g.genExprStmt(s)
	}
}

func (g *generator) genLoop(cond, step node, body *block) {
	startLabel, endLabel := g.newLabel(), g.newLabel()
	g.placeLabel(startLabel)
	if cond != nil {
		g.genBranch(cond, endLabel, false)
	}
	g.genBlock(body)
	if step != nil {
		g.genExprStmt(step)
	}
	g.emit(isa.OpJmp, labelImm(startLabel))
	g.placeLabel(endLabel)
}

func (g *generator) genExprStmt(e node) {
	switch e := e.(type) {
	case *assign:
		g.genAssign(e, false)
	case *call:
		if e.name == "in" { // in() без присваивания, символ просто отбрасываем
			g.free(g.genExpr(e))
			return
		}
		g.genCall(e, false)
	default:
		g.free(g.genExpr(e))
	}
}

func (g *generator) genReturn(s *returnStmt, jump bool) {
	if !g.inFunc {
		failf(s.line, "return outside of a function")
	}
	if s.value == nil {
		g.emit(isa.OpMov, reg(0), imm(0))
	} else if op, ok := g.leaf(s.value); ok {
		g.emit(isa.OpMov, reg(0), op)
	} else {
		r := g.genExpr(s.value)
		g.emit(isa.OpMov, reg(0), reg(r))
		g.free(r)
	}
	if jump {
		g.emit(isa.OpJmp, labelImm(g.exitLabel))
	}
}

func (g *generator) arraySize(d *varDecl) int32 {
	n, ok := g.constValue(d.size)
	if !ok || n <= 0 || n > isa.MemSize/4 {
		failf(d.line, "size of array %q must be a positive constant", d.name)
	}
	return n
}

func (g *generator) genVarDecl(d *varDecl) {
	if d.size != nil {
		n := g.arraySize(d)
		if g.inFunc {
			g.frame += n
			g.declare(d.name, d.line, symbol{kind: symLocalArr, value: -4 * g.frame})
			return
		}
		item := dataItem{label: fmt.Sprintf("arr%d", len(g.arrays)), words: make([]uint32, n)}
		for i := range n {
			item.notes = append(item.notes, fmt.Sprintf("%s[%d]", d.name, i))
		}
		g.arrays = append(g.arrays, item)
		g.declare(d.name, d.line, symbol{kind: symGlobalArr, label: item.label})
		return
	}

	// имя объявляем после инициализатора, так в auto x = x + 1 справа будет внешний x
	var s symbol
	var dst operand
	if g.inFunc {
		g.frame++
		s = symbol{kind: symLocal, value: -4 * g.frame}
		dst = fpOff(s.value)
	} else { // у кода верхнего уровня нет кадра, поэтому статика
		item := dataItem{label: fmt.Sprintf("var%d", len(g.vars)), words: []uint32{0}, notes: []string{d.name}}
		g.vars = append(g.vars, item)
		s = symbol{kind: symGlobal, label: item.label}
		dst = absLabel(item.label)
	}
	if d.init != nil {
		g.genStore(dst, "", d.init, false)
	}
	g.declare(d.name, d.line, s)
}

func (g *generator) localWords(s node) int32 {
	var n int32
	switch s := s.(type) {
	case *varDecl:
		if s.size != nil {
			return g.arraySize(s)
		}
		return 1
	case *block:
		for _, inner := range s.stmts {
			n += g.localWords(inner)
		}
	case *ifStmt:
		n = g.localWords(s.then)
		if s.els != nil {
			n += g.localWords(s.els)
		}
	case *whileStmt:
		n = g.localWords(s.body)
	case *forStmt:
		if s.init != nil {
			n = g.localWords(s.init)
		}
		n += g.localWords(s.body)
	}
	return n
}

func (g *generator) genFunc(f *funcDecl) {
	g.inFunc = true
	g.frame = 0
	g.exitLabel = g.newLabel()
	g.pushScope()
	for i, param := range f.params {
		off := int32(8 + 4*(len(f.params)-1-i)) // первый аргумент лежит выше всех
		g.declare(param, f.line, symbol{kind: symLocal, value: off})
	}

	g.placeLabel("func " + f.name)
	g.emit(isa.OpPush, reg(isa.RegFP))
	g.emit(isa.OpMov, reg(isa.RegFP), reg(isa.RegSP))
	if words := g.localWords(f.body); words > 0 { // место сразу под все локальные тела
		g.emit(isa.OpSub, reg(isa.RegSP), imm(4*words))
	}

	// тело в той же области видимости, что и параметры
	// последнему return прыгать на эпилог не нужно
	stmts := f.body.stmts
	endsWithReturn := false
	for i, s := range stmts {
		if ret, ok := s.(*returnStmt); ok && i == len(stmts)-1 {
			g.genReturn(ret, false)
			endsWithReturn = true
		} else {
			g.genStmt(s)
		}
	}
	if !endsWithReturn {
		g.emit(isa.OpMov, reg(0), imm(0)) // функция без return возвращает 0
	}

	g.placeLabel(g.exitLabel)
	g.emit(isa.OpMov, reg(isa.RegSP), reg(isa.RegFP))
	g.emit(isa.OpPop, reg(isa.RegFP))
	g.emit(isa.OpRet)
	g.popScope()
	g.inFunc = false
}

func (g *generator) genProgram(p *program) {
	// функции видны везде, поэтому сначала собираем все имена
	for _, item := range p.items {
		if f, ok := item.(*funcDecl); ok {
			if _, dup := g.funcArity[f.name]; dup {
				failf(f.line, "function %q is already declared", f.name)
			}
			if _, builtin := builtins[f.name]; builtin {
				failf(f.line, "%q is a built-in function name", f.name)
			}
			g.funcArity[f.name] = len(f.params)
		}
	}
	for _, item := range p.items {
		switch item := item.(type) {
		case *constDecl:
			v, ok := g.constValue(item.value)
			if !ok {
				failf(item.line, "value of constant %q is not known at translation time", item.name)
			}
			g.declare(item.name, item.line, symbol{kind: symConst, value: v})
		case *funcDecl:
			g.genFunc(item)
		default:
			g.genStmt(item)
		}
	}
	g.emit(isa.OpHalt) // после кода верхнего уровня, функции пойдут следом
}
