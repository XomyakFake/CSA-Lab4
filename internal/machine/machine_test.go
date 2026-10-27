package machine

import (
	"testing"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
)

func TestMicroprogramLayout(t *testing.T) {
	want := map[string]int{
		"FETCH": mFetch, "DECODE": mDecode, "O1_NONE": mO1None, "O1R_REG": mO1RReg,
		"O1R_IMM": mO1RImm, "O1R_ABS": mO1RAbs, "O1R_OFF": mO1ROff, "O1R_IND": mO1RInd,
		"O1R_MEM": mO1RMem, "O1W_ABS": mO1WAbs, "O1W_OFF": mO1WOff, "O1W_IND": mO1WInd,
		"O2_NONE": mO2None, "O2_REG": mO2Reg, "O2_IMM": mO2Imm, "O2_ABS": mO2Abs,
		"O2_OFF": mO2Off, "O2_IND": mO2Ind, "O2_MEM": mO2Mem, "WB_REG": mWBReg,
		"WB_MEM": mWBMem, "EX_ALU": mExAlu, "EX_CMP": mExCmp, "EX_MOV": mExMov,
		"EX_SET": mExSet, "EX_JCC": mExJcc, "EX_CALL": mExCall, "EX_RET": mExRet,
		"EX_PUSH": mExPush, "EX_POP": mExPop, "EX_HALT": mExHalt, "ERROR": mError,
	}
	seen := 0
	for addr, mi := range microprogram {
		if mi.label == "" {
			continue
		}
		seen++
		if want[mi.label] != addr {
			t.Errorf("%s is at address %d, tables expect %d", mi.label, addr, want[mi.label])
		}
	}
	if seen != len(want) {
		t.Errorf("microprogram has %d labels, want %d", seen, len(want))
	}
	if len(microprogram) != mError+1 {
		t.Errorf("microprogram has %d microinstructions, want %d", len(microprogram), mError+1)
	}
}

func TestDecodeFields(t *testing.T) {
	word := selRA(raSP) | selAluA(aluAPC) | selAluB(aluBCond) | aluOp(aluOpcode) |
		seq(seqRet) | selCond(condCntZero) | addr(0xAB) | condInv | halt
	s := decode(word) // если поля где-то пересекаются, значения не сойдутся
	if s.selRA != raSP || s.selAluA != aluAPC || s.selAluB != aluBCond || s.aluOp != aluOpcode ||
		s.seq != seqRet || s.selCond != condCntZero || s.addr != 0xAB || !s.condInv || !s.halt {
		t.Errorf("decoded fields do not match: %+v", s)
	}
	if s.latchPC || s.rfWr || s.imemRd || s.dmemWr || s.selPC != 0 {
		t.Errorf("unset signals are decoded as set: %+v", s)
	}
}

func run(t *testing.T, input string, ins ...isa.Instr) *Machine {
	t.Helper()
	var prog isa.Program
	for _, in := range ins {
		prog.Code = append(prog.Code, in.Encode()...)
	}
	prog.Code = append(prog.Code, isa.Instr{Op: isa.OpHalt}.Encode()...) // в конце всегда halt
	m := New(prog, []byte(input), true)
	m.Run(100_000)
	if m.StopReason != StopHalt {
		t.Fatalf("stop reason: %s", m.StopReason)
	}
	return m
}

func ins(op isa.Opcode, operands ...isa.Operand) isa.Instr {
	return isa.Instr{Op: op, Operands: operands}
}

func TestTicksPerInstruction(t *testing.T) {
	// числа из таблицы тактов в docs/processor.md
	const haltTicks = 5 // halt в конце программы, его такты вычитаем
	fp := isa.Reg(isa.RegFP)
	cases := []struct {
		name string
		in   isa.Instr
		want int
	}{
		{"add R1, R2", ins(isa.OpAdd, isa.Reg(1), isa.Reg(2)), 6},
		{"add [FP-4], #1", ins(isa.OpAdd, isa.Off(isa.RegFP, -4), isa.Imm(1)), 10},
		{"cmp [FP+8], #0", ins(isa.OpCmp, isa.Off(isa.RegFP, 8), isa.Imm(0)), 9},
		{"mov R1, R2", ins(isa.OpMov, isa.Reg(1), isa.Reg(2)), 6},
		{"mov [0x0084], #104", ins(isa.OpMov, isa.Abs(isa.PortOut), isa.Imm(104)), 8},
		{"slt R1", ins(isa.OpSlt, isa.Reg(1)), 6},
		{"jeq #8 (not taken)", ins(isa.OpJeq, isa.Imm(8)), 6},
		{"jmp #8 (taken)", ins(isa.OpJmp, isa.Imm(8)), 7},
		{"push FP", ins(isa.OpPush, fp), 10},
		{"push R1, #5", ins(isa.OpPush, isa.Reg(1), isa.Imm(5)), 16},
		{"neg [R1]", ins(isa.OpNeg, isa.Ind(1)), 2 + 3 + 1 + 2},
		{"mov [R1], [0x0100]", ins(isa.OpMov, isa.Ind(1), isa.Abs(0x100)), 2 + 1 + 4 + 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// FP и R1 указывают в память, чтобы адреса операндов были допустимыми
			m := New(isa.Program{Code: append(c.in.Encode(), 0)}, nil, true)
			m.DP.RF[isa.RegFP] = 0x8000
			m.DP.RF[1] = 0x4000
			m.Run(100_000)
			if got := m.Ticks - m.WaitTicks - haltTicks; got != c.want { // ожидание кеша не считаем
				t.Errorf("got %d ticks, want %d", got, c.want)
			}
		})
	}
}

func TestCallRetPushPop(t *testing.T) {
	// call на 0x10, потом jmp на halt, а по 0x10 лежат push FP, pop FP, ret
	// такты 8 + 7 + 10 + 13 + 7 и ещё 5 на halt
	prog := isa.Program{}
	for _, in := range []isa.Instr{
		ins(isa.OpCall, isa.Imm(0x10)),
		ins(isa.OpJmp, isa.Imm(0x24)),
		ins(isa.OpPush, isa.Reg(isa.RegFP)),
		ins(isa.OpPop, isa.Reg(isa.RegFP)),
		ins(isa.OpRet),
		ins(isa.OpHalt),
	} {
		prog.Code = append(prog.Code, in.Encode()...)
	}
	m := New(prog, nil, true)
	m.DP.RF[isa.RegFP] = 0x1234
	m.Run(100_000)
	if m.StopReason != StopHalt {
		t.Fatalf("stop reason: %s", m.StopReason)
	}
	if got, want := m.Ticks-m.WaitTicks, 8+10+13+7+7+5; got != want {
		t.Errorf("got %d ticks, want %d", got, want)
	}
	if m.DP.RF[isa.RegSP] != isa.MemSize || m.DP.RF[isa.RegFP] != 0x1234 {
		t.Errorf("SP=%X FP=%X after balanced call/ret and push/pop", m.DP.RF[isa.RegSP], m.DP.RF[isa.RegFP])
	}
}

func TestCarryChain(t *testing.T) {
	m := run(t, "",
		ins(isa.OpMov, isa.Reg(1), isa.Imm(-1)), // младшее слово 0xFFFFFFFF
		ins(isa.OpMov, isa.Reg(2), isa.Imm(1)),  // старшее слово 1
		ins(isa.OpAdd, isa.Reg(1), isa.Imm(1)),  // перенос
		ins(isa.OpAdc, isa.Reg(2), isa.Imm(0)),
		ins(isa.OpMov, isa.Reg(3), isa.Imm(0)), // 0x00000001_00000000 - 1
		ins(isa.OpMov, isa.Reg(4), isa.Imm(1)),
		ins(isa.OpSub, isa.Reg(3), isa.Imm(1)), // заём
		ins(isa.OpSbc, isa.Reg(4), isa.Imm(0)),
	)
	rf := m.DP.RF
	if rf[1] != 0 || rf[2] != 2 {
		t.Errorf("add/adc: got %08X_%08X, want 00000002_00000000", rf[2], rf[1])
	}
	if rf[3] != 0xFFFFFFFF || rf[4] != 0 {
		t.Errorf("sub/sbc: got %08X_%08X, want 00000000_FFFFFFFF", rf[4], rf[3])
	}
}

func TestConditions(t *testing.T) {
	cases := []struct {
		a, b int32
		set  isa.Opcode
		want uint32
	}{
		{1, 2, isa.OpSlt, 1},
		{2, 1, isa.OpSlt, 0},
		{-1, 1, isa.OpSlt, 1},
		{-2147483648, 1, isa.OpSlt, 1}, // тут переполнение, поэтому смотрим N != V
		{5, 5, isa.OpSle, 1},
		{5, 5, isa.OpSgt, 0},
		{7, -3, isa.OpSgt, 1},
		{7, 7, isa.OpSge, 1},
		{3, 3, isa.OpSeq, 1},
		{3, 4, isa.OpSne, 1},
	}
	for _, c := range cases {
		m := run(t, "",
			ins(isa.OpMov, isa.Reg(1), isa.Imm(c.a)),
			ins(isa.OpCmp, isa.Reg(1), isa.Imm(c.b)),
			ins(c.set, isa.Reg(2)),
		)
		if m.DP.RF[2] != c.want {
			t.Errorf("cmp %d, %d then %s: got %d, want %d", c.a, c.b, c.set.Name(), m.DP.RF[2], c.want)
		}
	}

	// jcs это беззнаковое a < b, а 0xFFFFFFFF больше 1, так что перехода нет
	m := run(t, "",
		ins(isa.OpMov, isa.Reg(1), isa.Imm(-1)),
		ins(isa.OpCmp, isa.Reg(1), isa.Imm(1)),
		ins(isa.OpJcs, isa.Imm(0x100)), // переход увёл бы на пустую память (halt)
		ins(isa.OpMov, isa.Reg(2), isa.Imm(7)),
	)
	if m.DP.RF[2] != 7 {
		t.Error("jcs taken for unsigned 0xFFFFFFFF < 1")
	}
}

func TestStopReasons(t *testing.T) {
	cases := []struct {
		name string
		in   isa.Instr
		want string
	}{
		{"division by zero", ins(isa.OpDiv, isa.Reg(1), isa.Imm(0)), StopDivZero},
		{"input exhausted", ins(isa.OpMov, isa.Reg(1), isa.Abs(isa.PortIn)), StopInputExhausted},
		{"unaligned address", ins(isa.OpMov, isa.Reg(1), isa.Abs(0x101)), StopBadAddress},
		{"immediate destination", ins(isa.OpMov, isa.Imm(1), isa.Reg(1)), StopBadInstruction},
		{"unknown opcode", isa.Instr{Op: 0x7F}, StopBadInstruction},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := New(isa.Program{Code: c.in.Encode()}, nil, true)
			m.Run(100_000)
			if m.StopReason != c.want {
				t.Errorf("got %q, want %q", m.StopReason, c.want)
			}
		})
	}
}

func TestCacheTiming(t *testing.T) {
	mem := make([]uint32, isa.MemSize/4)
	mem[0x100/4] = 11
	mem[0x200/4] = 22
	c := newCache(mem, true)
	access := func(addr uint32, write bool, wdata uint32) (ticks int, rdata uint32) {
		for {
			ticks++
			ready, data, _ := c.Access(addr, write, wdata)
			if ready {
				return ticks, data
			}
		}
	}
	if ticks, data := access(0x100, false, 0); ticks != 10 || data != 11 {
		t.Errorf("miss: %d ticks, data %d", ticks, data)
	}
	if ticks, _ := access(0x104, false, 0); ticks != 1 {
		t.Errorf("hit in the same line: %d ticks", ticks)
	}
	if ticks, _ := access(0x108, true, 33); ticks != 1 {
		t.Errorf("write hit: %d ticks", ticks)
	}
	if mem[0x108/4] != 0 {
		t.Error("write-back cache must not write memory on a hit")
	}
	// 0x200 попадает в ту же строку 0, но тег другой, а строка грязная
	if ticks, data := access(0x200, false, 0); ticks != 20 || data != 22 {
		t.Errorf("miss with dirty eviction: %d ticks, data %d", ticks, data)
	}
	if mem[0x108/4] != 33 {
		t.Error("dirty line was not written back on eviction")
	}
	if ticks, data := access(0x108, false, 0); ticks != 10 || data != 33 {
		t.Errorf("miss with clean eviction: %d ticks, data %d", ticks, data)
	}

	direct := newCache(mem, false)
	c = direct
	if ticks, data := access(0x100, false, 0); ticks != 10 || data != 11 {
		t.Errorf("no cache: %d ticks, data %d", ticks, data)
	}
	if ticks, _ := access(0x100, false, 0); ticks != 10 {
		t.Errorf("no cache, repeated access: %d ticks", ticks)
	}
}

func TestPortsShareCacheLineWithData(t *testing.T) {
	// порты 0x80 и 0x84 в одной строке кеша с данными 0x88 и 0x8C
	m := run(t, "AB",
		ins(isa.OpMov, isa.Abs(0x88), isa.Imm(5)),           // строка 0x80..0x8F попадает в кеш
		ins(isa.OpMov, isa.Reg(1), isa.Abs(isa.PortIn)),     // 'A'
		ins(isa.OpMov, isa.Abs(0x8C), isa.Abs(isa.PortIn)),  // 'B' в соседнее слово
		ins(isa.OpMov, isa.Abs(isa.PortOut), isa.Abs(0x8C)), // вывести 'B'
		ins(isa.OpMov, isa.Abs(isa.PortOut), isa.Reg(1)),    // вывести 'A'
		ins(isa.OpAdd, isa.Abs(0x88), isa.Abs(0x8C)),        // 5 + 'B'
		ins(isa.OpMov, isa.Reg(2), isa.Abs(0x88)),
	)
	if string(m.IO.Output) != "BA" {
		t.Errorf("output %q, want %q", m.IO.Output, "BA")
	}
	if m.DP.RF[2] != 5+'B' {
		t.Errorf("data next to ports: got %d, want %d", m.DP.RF[2], 5+'B')
	}
}
