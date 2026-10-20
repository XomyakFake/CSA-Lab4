package isa

import (
	"fmt"
	"strings"
)

type Opcode uint8

const (
	OpHalt Opcode = 0x00
	OpMov  Opcode = 0x01
	OpPush Opcode = 0x02
	OpPop  Opcode = 0x03

	OpAdd Opcode = 0x10 // у add..not младшие 4 бита это код операции АЛУ
	OpSub Opcode = 0x11
	OpMul Opcode = 0x12
	OpDiv Opcode = 0x13
	OpRem Opcode = 0x14
	OpAnd Opcode = 0x15
	OpOr  Opcode = 0x16
	OpAdc Opcode = 0x17
	OpSbc Opcode = 0x18
	OpCmp Opcode = 0x19
	OpNeg Opcode = 0x1A
	OpNot Opcode = 0x1B

	OpJmp Opcode = 0x20 // у переходов младшие 4 бита это код условия, у seq..sge такие же
	OpJeq Opcode = 0x21
	OpJne Opcode = 0x22
	OpJlt Opcode = 0x23
	OpJle Opcode = 0x24
	OpJgt Opcode = 0x25
	OpJge Opcode = 0x26
	OpJcs Opcode = 0x27
	OpJcc Opcode = 0x28

	OpSeq Opcode = 0x31
	OpSne Opcode = 0x32
	OpSlt Opcode = 0x33
	OpSle Opcode = 0x34
	OpSgt Opcode = 0x35
	OpSge Opcode = 0x36

	OpCall Opcode = 0x40
	OpRet  Opcode = 0x41
)

type Arity int

const (
	Arity0 Arity = iota
	Arity1
	Arity2
	ArityVar // push и pop
)

type opInfo struct {
	name  string
	arity Arity
}

var opTable = map[Opcode]opInfo{
	OpHalt: {"halt", Arity0},
	OpMov:  {"mov", Arity2},
	OpPush: {"push", ArityVar},
	OpPop:  {"pop", ArityVar},
	OpAdd:  {"add", Arity2},
	OpSub:  {"sub", Arity2},
	OpMul:  {"mul", Arity2},
	OpDiv:  {"div", Arity2},
	OpRem:  {"rem", Arity2},
	OpAnd:  {"and", Arity2},
	OpOr:   {"or", Arity2},
	OpAdc:  {"adc", Arity2},
	OpSbc:  {"sbc", Arity2},
	OpCmp:  {"cmp", Arity2},
	OpNeg:  {"neg", Arity1},
	OpNot:  {"not", Arity1},
	OpJmp:  {"jmp", Arity1},
	OpJeq:  {"jeq", Arity1},
	OpJne:  {"jne", Arity1},
	OpJlt:  {"jlt", Arity1},
	OpJle:  {"jle", Arity1},
	OpJgt:  {"jgt", Arity1},
	OpJge:  {"jge", Arity1},
	OpJcs:  {"jcs", Arity1},
	OpJcc:  {"jcc", Arity1},
	OpSeq:  {"seq", Arity1},
	OpSne:  {"sne", Arity1},
	OpSlt:  {"slt", Arity1},
	OpSle:  {"sle", Arity1},
	OpSgt:  {"sgt", Arity1},
	OpSge:  {"sge", Arity1},
	OpCall: {"call", Arity1},
	OpRet:  {"ret", Arity0},
}

func (op Opcode) Name() string {
	if info, ok := opTable[op]; ok {
		return info.name
	}
	return fmt.Sprintf("op_%02X", uint8(op))
}

func (op Opcode) Arity() Arity {
	return opTable[op].arity
}

func (op Opcode) Valid() bool {
	_, ok := opTable[op]
	return ok
}

func (op Opcode) IsJump() bool {
	return (op >= OpJmp && op <= OpJcc) || op == OpCall // операнд у них это адрес в памяти команд
}

type Mode uint8

const (
	ModeNone Mode = 0 // нет операнда
	ModeReg  Mode = 1 // Rn
	ModeImm  Mode = 2 // #value
	ModeAbs  Mode = 3 // [addr]
	ModeInd  Mode = 4 // [Rn]
	ModeOff  Mode = 5 // [Rn+off]
)

func (m Mode) HasExt() bool {
	return m == ModeImm || m == ModeAbs || m == ModeOff
}

const (
	RegFP    = 8
	RegSP    = 9
	NumRegs  = 10
	PoolSize = 7 // R1..R7 под временные значения выражений
)

func RegName(r uint8) string {
	switch r {
	case RegFP:
		return "FP"
	case RegSP:
		return "SP"
	default:
		return fmt.Sprintf("R%d", r)
	}
}

const (
	PortIn     = 0x80
	PortOut    = 0x84
	StaticBase = 0x88    // статика сразу после портов
	MemSize    = 0x10000 // по 64 КиБ на каждую память
)

type Operand struct {
	Mode  Mode
	Reg   uint8
	Value int32 // слово расширения, то есть число, адрес или смещение
}

func Reg(r uint8) Operand { return Operand{Mode: ModeReg, Reg: r} }

func Imm(v int32) Operand { return Operand{Mode: ModeImm, Value: v} }

func Abs(addr int32) Operand { return Operand{Mode: ModeAbs, Value: addr} }

func Ind(r uint8) Operand { return Operand{Mode: ModeInd, Reg: r} }

func Off(r uint8, off int32) Operand { return Operand{Mode: ModeOff, Reg: r, Value: off} }

func (o Operand) IsMem() bool {
	return o.Mode == ModeAbs || o.Mode == ModeInd || o.Mode == ModeOff
}

func (o Operand) Format(jump bool) string {
	switch o.Mode {
	case ModeNone:
		return ""
	case ModeReg:
		return RegName(o.Reg)
	case ModeImm:
		if jump { // адрес перехода удобнее читать в hex
			return fmt.Sprintf("#0x%04X", uint32(o.Value))
		}
		return fmt.Sprintf("#%d", o.Value)
	case ModeAbs:
		return fmt.Sprintf("[0x%04X]", uint32(o.Value))
	case ModeInd:
		return "[" + RegName(o.Reg) + "]"
	case ModeOff:
		return fmt.Sprintf("[%s%+d]", RegName(o.Reg), o.Value)
	default:
		return fmt.Sprintf("?mode%d", o.Mode)
	}
}

type Instr struct {
	Op       Opcode
	Operands []Operand
}

func (in Instr) String() string {
	if len(in.Operands) == 0 {
		return in.Op.Name()
	}
	parts := make([]string, len(in.Operands))
	for i, o := range in.Operands {
		parts[i] = o.Format(in.Op.IsJump())
	}
	return in.Op.Name() + " " + strings.Join(parts, ", ")
}

func (in Instr) Size() int {
	n := 1 // первое слово, дальше считаем слова операндов
	for _, o := range in.Operands {
		if in.Op.Arity() == ArityVar {
			n++ // слово-описатель операнда
		}
		if o.Mode.HasExt() {
			n++
		}
	}
	return n
}
