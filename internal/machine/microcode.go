package machine

import (
	"strconv"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
)

const ( // где лежат поля в 64-битном слове микрокоманды, как в docs/processor.md
	shSelRA   = 0 // 2 бита
	shSelAluA = 2 // 2 бита
	shSelAluB = 4 // 3 бита
	shAluOp   = 7 // 4 бита

	bitSelPC       = 11
	bitSelWA       = 12
	bitSelDAddr    = 13
	bitLatchPC     = 14
	bitLatchIR     = 15
	bitLatchIRDesc = 16
	bitLatchEXT    = 17
	bitLatchOPA    = 18
	bitLatchOPB    = 19
	bitLatchADST   = 20
	bitLatchAR     = 21
	bitLatchDR     = 22
	bitLatchFlags  = 23
	bitLatchCnt    = 24
	bitDecCnt      = 25
	bitRFWr        = 26
	bitIMemRd      = 27
	bitDMemRd      = 28
	bitDMemWr      = 29
	bitHalt        = 30

	shSeq      = 31 // 3 бита
	shSelCond  = 34 // 2 бита
	bitCondInv = 36
	shAddr     = 37 // 8 бит
)

const ( // входы мультиплексоров, имена как на схеме
	raR1 = 0 // MUX_RA
	raR2 = 1
	raSP = 2 // константа 9, это номер SP

	waR1 = 0 // MUX_WA
	waSP = 1

	aluARF  = 0 // MUX_ALU_A
	aluAOPA = 1
	aluAPC  = 2

	aluBOPB  = 0 // MUX_ALU_B
	aluBEXT  = 1
	aluBDR   = 2
	aluB4    = 3
	aluBCond = 4 // выход блока COND, нужен для seq..sge

	pcPlus4 = 0 // MUX_PC
	pcAlu   = 1

	daddrAR   = 0 // MUX_DADDR
	daddrADST = 1

	condTrue    = 0 // MUX_COND
	condCntZero = 1
)

const (
	aluAdd    = 0x0 // 0x0..0xB совпадают с младшими битами опкодов add..not
	aluSub    = 0x1
	aluMul    = 0x2
	aluDiv    = 0x3
	aluRem    = 0x4
	aluAnd    = 0x5
	aluOr     = 0x6
	aluAdc    = 0x7
	aluSbc    = 0x8
	aluCmp    = 0x9
	aluNeg    = 0xA
	aluNot    = 0xB
	aluPassA  = 0xC
	aluPassB  = 0xD
	aluOpcode = 0xF // операция берётся из младших 4 бит опкода в IR
)

const ( // поле seq, куда идти после микрокоманды
	seqNext   = 0 // mpmPC + 1
	seqJmp    = 1 // addr
	seqBr     = 2 // addr, если условие истинно, иначе mpmPC + 1
	seqDecode = 3 // ModeMPM1[wo, M1], mpmRET <- OpMPM[opcode]
	seqOp2    = 4 // ModeMPM2[M2]
	seqCall1  = 5 // ModeMPM1[wo, M1], mpmRET <- addr
	seqWB     = 6 // WBMPM[M1], mpmRET <- addr
	seqRet    = 7 // mpmRET
)

const (
	mFetch  = 0 // адреса в MPM
	mDecode = 1
	mO1None = 2
	mO1RReg = 3
	mO1RImm = 4
	mO1RAbs = 6
	mO1ROff = 8
	mO1RInd = 10
	mO1RMem = 11
	mO1WAbs = 13
	mO1WOff = 15
	mO1WInd = 17
	mO2None = 18
	mO2Reg  = 19
	mO2Imm  = 20
	mO2Abs  = 22
	mO2Off  = 24
	mO2Ind  = 26
	mO2Mem  = 27
	mWBReg  = 29
	mWBMem  = 30
	mExAlu  = 31
	mExCmp  = 32
	mExMov  = 33
	mExSet  = 34
	mExJcc  = 35
	mExCall = 37
	mExRet  = 40
	mExPush = 43
	mExPop  = 47
	mExHalt = 53
	mError  = 54
)

func bit(n uint) uint64 { return 1 << n }

func selRA(v uint64) uint64   { return v << shSelRA }
func selAluA(v uint64) uint64 { return v << shSelAluA }
func selAluB(v uint64) uint64 { return v << shSelAluB }
func aluOp(v uint64) uint64   { return v << shAluOp }
func seq(v uint64) uint64     { return v << shSeq }
func selCond(v uint64) uint64 { return v << shSelCond }
func addr(v uint64) uint64    { return v << shAddr }

const (
	selPCAlu     = pcAlu << bitSelPC
	selWASP      = waSP << bitSelWA
	selDAddrADST = daddrADST << bitSelDAddr
	condInv      = 1 << bitCondInv
)

var (
	latchPC     = bit(bitLatchPC)
	latchIR     = bit(bitLatchIR)
	latchIRDesc = bit(bitLatchIRDesc)
	latchEXT    = bit(bitLatchEXT)
	latchOPA    = bit(bitLatchOPA)
	latchOPB    = bit(bitLatchOPB)
	latchADST   = bit(bitLatchADST)
	latchAR     = bit(bitLatchAR)
	latchDR     = bit(bitLatchDR)
	latchFlags  = bit(bitLatchFlags)
	latchCnt    = bit(bitLatchCnt)
	decCnt      = bit(bitDecCnt)
	rfWr        = bit(bitRFWr)
	imemRd      = bit(bitIMemRd)
	dmemRd      = bit(bitDMemRd)
	dmemWr      = bit(bitDMemWr)
	halt        = bit(bitHalt)

	fetchExt       = imemRd | latchEXT | latchPC                                      // EXT <- IMem[PC], PC <- PC + 4
	fromEXT        = selAluB(aluBEXT) | aluOp(aluPassB)                               // alu_out = EXT
	fromDR         = selAluB(aluBDR) | aluOp(aluPassB)                                // alu_out = DR
	fromOPA        = selAluA(aluAOPA) | aluOp(aluPassA)                               // alu_out = OPA
	fromR1         = selRA(raR1) | selAluA(aluARF) | aluOp(aluPassA)                  // alu_out = RF[R1]
	r1PlusEx       = selRA(raR1) | selAluA(aluARF) | selAluB(aluBEXT) | aluOp(aluAdd) // alu_out = RF[R1] + EXT
	fromR2         = selRA(raR2) | selAluA(aluARF) | aluOp(aluPassA)                  // alu_out = RF[R2]
	r2PlusEx       = selRA(raR2) | selAluA(aluARF) | selAluB(aluBEXT) | aluOp(aluAdd) // alu_out = RF[R2] + EXT
	fromSP         = selRA(raSP) | selAluA(aluARF) | aluOp(aluPassA)                  // alu_out = SP
	spMinus4       = selRA(raSP) | selAluA(aluARF) | selAluB(aluB4) | aluOp(aluSub)   // alu_out = SP - 4
	spPlus4        = selRA(raSP) | selAluA(aluARF) | selAluB(aluB4) | aluOp(aluAdd)   // alu_out = SP + 4
	writeSP        = selWASP | rfWr                                                   // SP <- alu_out
	fetchDesc      = imemRd | latchIRDesc | latchPC | decCnt                          // описатель операнда push/pop идёт только в поле M1 R1
	brMoreOperands = seq(seqBr) | selCond(condCntZero) | condInv                      // крутим цикл push/pop, пока CNT не станет 0
)

type microinstruction struct {
	label string // нужна только для журнала
	word  uint64
}

var microprogram = []microinstruction{ // номер строки это адрес в MPM, таблица та же что в docs/processor.md
	/*  0 */ {"FETCH", imemRd | latchIR | latchPC | seq(seqNext)},
	/*  1 */ {"DECODE", latchCnt | seq(seqDecode)},

	// первый операнд, значение читаем в OPA
	/*  2 */ {"O1_NONE", seq(seqOp2)},
	/*  3 */ {"O1R_REG", fromR1 | latchOPA | seq(seqOp2)},
	/*  4 */ {"O1R_IMM", fetchExt | seq(seqNext)},
	/*  5 */ {"", fromEXT | latchOPA | seq(seqOp2)},
	/*  6 */ {"O1R_ABS", fetchExt | seq(seqNext)},
	/*  7 */ {"", fromEXT | latchADST | seq(seqJmp) | addr(mO1RMem)},
	/*  8 */ {"O1R_OFF", fetchExt | seq(seqNext)},
	/*  9 */ {"", r1PlusEx | latchADST | seq(seqJmp) | addr(mO1RMem)},
	/* 10 */ {"O1R_IND", fromR1 | latchADST | seq(seqNext)},
	/* 11 */ {"O1R_MEM", dmemRd | selDAddrADST | latchDR | seq(seqNext)},
	/* 12 */ {"", fromDR | latchOPA | seq(seqOp2)},

	// первый операнд только как приёмник, нужен лишь адрес в ADST
	/* 13 */ {"O1W_ABS", fetchExt | seq(seqNext)},
	/* 14 */ {"", fromEXT | latchADST | seq(seqOp2)},
	/* 15 */ {"O1W_OFF", fetchExt | seq(seqNext)},
	/* 16 */ {"", r1PlusEx | latchADST | seq(seqOp2)},
	/* 17 */ {"O1W_IND", fromR1 | latchADST | seq(seqOp2)},

	// второй операнд в OPB
	/* 18 */ {"O2_NONE", seq(seqRet)},
	/* 19 */ {"O2_REG", fromR2 | latchOPB | seq(seqRet)},
	/* 20 */ {"O2_IMM", fetchExt | seq(seqNext)},
	/* 21 */ {"", fromEXT | latchOPB | seq(seqRet)},
	/* 22 */ {"O2_ABS", fetchExt | seq(seqNext)},
	/* 23 */ {"", fromEXT | latchAR | seq(seqJmp) | addr(mO2Mem)},
	/* 24 */ {"O2_OFF", fetchExt | seq(seqNext)},
	/* 25 */ {"", r2PlusEx | latchAR | seq(seqJmp) | addr(mO2Mem)},
	/* 26 */ {"O2_IND", fromR2 | latchAR | seq(seqNext)},
	/* 27 */ {"O2_MEM", dmemRd | latchDR | seq(seqNext)},
	/* 28 */ {"", fromDR | latchOPB | seq(seqRet)},

	// запись результата из OPA
	/* 29 */ {"WB_REG", fromOPA | rfWr | seq(seqRet)},
	/* 30 */ {"WB_MEM", fromOPA | dmemWr | selDAddrADST | seq(seqRet)},

	// исполнение команд
	/* 31 */ {"EX_ALU", selAluA(aluAOPA) | aluOp(aluOpcode) | latchOPA | latchFlags | seq(seqWB) | addr(mFetch)},
	/* 32 */ {"EX_CMP", selAluA(aluAOPA) | aluOp(aluOpcode) | latchFlags | seq(seqJmp) | addr(mFetch)},
	/* 33 */ {"EX_MOV", aluOp(aluPassB) | latchOPA | seq(seqWB) | addr(mFetch)},
	/* 34 */ {"EX_SET", selAluB(aluBCond) | aluOp(aluPassB) | latchOPA | seq(seqWB) | addr(mFetch)},
	/* 35 */ {"EX_JCC", seq(seqBr) | selCond(condTrue) | condInv | addr(mFetch)},
	/* 36 */ {"", fromOPA | selPCAlu | latchPC | seq(seqJmp) | addr(mFetch)},
	/* 37 */ {"EX_CALL", spMinus4 | writeSP | latchAR | seq(seqNext)},
	/* 38 */ {"", selAluA(aluAPC) | aluOp(aluPassA) | dmemWr | seq(seqNext)},
	/* 39 */ {"", fromOPA | selPCAlu | latchPC | seq(seqJmp) | addr(mFetch)},
	/* 40 */ {"EX_RET", fromSP | latchAR | seq(seqNext)},
	/* 41 */ {"", dmemRd | latchDR | spPlus4 | writeSP | seq(seqNext)},
	/* 42 */ {"", fromDR | selPCAlu | latchPC | seq(seqJmp) | addr(mFetch)},
	/* 43 */ {"EX_PUSH", fetchDesc | seq(seqNext)},
	/* 44 */ {"", spMinus4 | writeSP | latchAR | seq(seqCall1) | addr(mExPush+2)},
	/* 45 */ {"", fromOPA | dmemWr | brMoreOperands | addr(mExPush)},
	/* 46 */ {"", seq(seqJmp) | addr(mFetch)},
	/* 47 */ {"EX_POP", fetchDesc | seq(seqNext)},
	/* 48 */ {"", fromSP | latchAR | seq(seqCall1) | addr(mExPop+2)},
	/* 49 */ {"", dmemRd | latchDR | spPlus4 | writeSP | seq(seqNext)},
	/* 50 */ {"", fromDR | latchOPA | seq(seqWB) | addr(mExPop+4)},
	/* 51 */ {"", brMoreOperands | addr(mExPop)},
	/* 52 */ {"", seq(seqJmp) | addr(mFetch)},
	/* 53 */ {"EX_HALT", halt},
	/* 54 */ {"ERROR", halt},
}

const mpmSize = 256 // адрес микрокоманды 8 бит

func buildMPM() [mpmSize]uint64 {
	var memory [mpmSize]uint64
	for i := range memory {
		memory[i] = halt // если прыгнем в пустую ячейку, процессор встанет и это будет видно
	}
	for i, mi := range microprogram {
		memory[i] = mi.word
	}
	return memory
}

func microLabel(mpc uint8) string {
	if int(mpc) >= len(microprogram) {
		return "?"
	}
	for i := int(mpc); i >= 0; i-- {
		if microprogram[i].label != "" {
			if i == int(mpc) {
				return microprogram[i].label
			}
			return microprogram[i].label + "+" + strconv.Itoa(int(mpc)-i) // например O1R_OFF+1
		}
	}
	return "?"
}

type opEntry struct {
	addr uint8
	wo   uint8 // 1 если первый операнд только приёмник и читать его не надо
}

func buildOpMPM() [256]opEntry {
	var t [256]opEntry
	for i := range t {
		t[i] = opEntry{addr: mError} // неизвестный опкод ведёт на ERROR
	}
	t[isa.OpHalt] = opEntry{mExHalt, 0}
	t[isa.OpMov] = opEntry{mExMov, 1}
	t[isa.OpPush] = opEntry{mExPush, 0}
	t[isa.OpPop] = opEntry{mExPop, 1}
	for op := isa.OpAdd; op <= isa.OpNot; op++ {
		t[op] = opEntry{mExAlu, 0}
	}
	t[isa.OpCmp] = opEntry{mExCmp, 0}
	for op := isa.OpJmp; op <= isa.OpJcc; op++ {
		t[op] = opEntry{mExJcc, 0}
	}
	for op := isa.OpSeq; op <= isa.OpSge; op++ {
		t[op] = opEntry{mExSet, 1}
	}
	t[isa.OpCall] = opEntry{mExCall, 0}
	t[isa.OpRet] = opEntry{mExRet, 0}
	return t
}

type modeTable [16]uint8 // индекс это код режима адресации

func buildModeTable(entries ...uint8) modeTable {
	var t modeTable
	for i := range t {
		t[i] = mError
	}
	copy(t[:], entries)
	return t
}

var (
	mpm      = buildMPM()
	opMPM    = buildOpMPM()
	modeMPM1 = [2]modeTable{ // режимы по порядку 0 нет, 1 Rn, 2 #v, 3 [addr], 4 [Rn], 5 [Rn+off]
		buildModeTable(mO1None, mO1RReg, mO1RImm, mO1RAbs, mO1RInd, mO1ROff), // wo = 0
		buildModeTable(mO1None, mO1None, mError, mO1WAbs, mO1WInd, mO1WOff),  // wo = 1
	}
	modeMPM2 = buildModeTable(mO2None, mO2Reg, mO2Imm, mO2Abs, mO2Ind, mO2Off)
	wbMPM    = buildModeTable(mError, mWBReg, mError, mWBMem, mWBMem, mWBMem)
)
