package machine

import "github.com/XomyakFake/CSA-Lab4/internal/isa"

type Flags struct {
	N, Z, V, C bool
}

type DataPath struct {
	PC   uint32
	IR   uint32 // opcode | M1 | R1 | M2 | R2 | N
	EXT  uint32
	RF   [isa.NumRegs]uint32 // R0..R7, FP, SP
	OPA  uint32
	OPB  uint32
	ADST uint32
	AR   uint32
	DR   uint32
	Flags
	CNT uint8
}

func (dp *DataPath) opcode() uint8 { return uint8(dp.IR >> 24) }
func (dp *DataPath) m1() uint8     { return uint8(dp.IR>>20) & 0xF }
func (dp *DataPath) r1() uint8     { return uint8(dp.IR>>16) & 0xF }
func (dp *DataPath) m2() uint8     { return uint8(dp.IR>>12) & 0xF }
func (dp *DataPath) r2() uint8     { return uint8(dp.IR>>8) & 0xF }
func (dp *DataPath) n() uint8      { return uint8(dp.IR) }

const irDescMask = 0x00FF0000 // поле M1 R1, только его пишет latch_ir_desc

func (dp *DataPath) readAddr(selRA uint8) uint8 {
	switch selRA { // MUX_RA
	case raR1:
		return dp.r1()
	case raR2:
		return dp.r2()
	default:
		return isa.RegSP
	}
}

func (dp *DataPath) writeAddr(selWA uint8) uint8 {
	if selWA == waR1 { // MUX_WA
		return dp.r1()
	}
	return isa.RegSP
}

func (dp *DataPath) rfOut(readAddr uint8) uint32 {
	if readAddr >= isa.NumRegs { // регистров всего 10, лишний номер читаем как 0
		return 0
	}
	return dp.RF[readAddr]
}

func (dp *DataPath) aluA(selAluA, selRA uint8) uint32 {
	switch selAluA { // MUX_ALU_A
	case aluARF:
		return dp.rfOut(dp.readAddr(selRA))
	case aluAOPA:
		return dp.OPA
	default:
		return dp.PC
	}
}

func (dp *DataPath) aluB(selAluB uint8, cond bool) uint32 {
	switch selAluB { // MUX_ALU_B
	case aluBOPB:
		return dp.OPB
	case aluBEXT:
		return dp.EXT
	case aluBDR:
		return dp.DR
	case aluB4:
		return 4
	default: // вход COND, это 0 или 1 для seq..sge
		if cond {
			return 1
		}
		return 0
	}
}

func (dp *DataPath) address(selDAddr uint8) uint32 {
	if selDAddr == daddrADST { // MUX_DADDR
		return dp.ADST
	}
	return dp.AR
}

func alu(op uint8, a, b uint32, carry bool) (out uint32, f Flags, divZero bool) {
	var c uint32
	if carry {
		c = 1
	}
	// V при сложении, когда слагаемые одного знака, а сумма другого
	// V при вычитании, когда знаки операндов разные и разность не того знака, что уменьшаемое
	addOverflow := func() bool { return (a^out)&(b^out)&0x80000000 != 0 }
	subOverflow := func() bool { return (a^b)&(a^out)&0x80000000 != 0 }

	switch op {
	case aluAdd:
		out = a + b
		f.C = out < a // беззнаковый перенос
		f.V = addOverflow()
	case aluSub, aluCmp:
		out = a - b
		f.C = a < b // заём, он же беззнаковое a < b
		f.V = subOverflow()
	case aluMul:
		out = uint32(int32(a) * int32(b))
	case aluDiv, aluRem:
		if b == 0 {
			return 0, f, true // div_zero остановит процессор
		}
		if op == aluDiv {
			out = uint32(int32(a) / int32(b))
		} else {
			out = uint32(int32(a) % int32(b))
		}
	case aluAnd:
		out = a & b
	case aluOr:
		out = a | b
	case aluAdc:
		sum := uint64(a) + uint64(b) + uint64(c)
		out = uint32(sum)
		f.C = sum>>32 != 0
		f.V = addOverflow()
	case aluSbc:
		out = a - b - c
		f.C = uint64(a) < uint64(b)+uint64(c)
		f.V = subOverflow()
	case aluNeg:
		out = -a
		f.C = a != 0
		f.V = a == 0x80000000
	case aluNot:
		out = ^a
	case aluPassA:
		out = a
	case aluPassB:
		out = b
	}
	f.N = out&0x80000000 != 0
	f.Z = out == 0
	return out, f, false
}

func condBlock(code uint8, f Flags) bool {
	switch code { // код условия это младшие 4 бита опкода
	case 0:
		return true
	case 1:
		return f.Z
	case 2:
		return !f.Z
	case 3:
		return f.N != f.V
	case 4:
		return f.Z || f.N != f.V
	case 5:
		return !f.Z && f.N == f.V
	case 6:
		return f.N == f.V
	case 7:
		return f.C
	case 8:
		return !f.C
	default:
		return false
	}
}
