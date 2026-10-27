package machine

type signals struct { // микрокоманда, разложенная на сигналы
	selRA, selAluA, selAluB, aluOp uint8
	selPC, selWA, selDAddr         uint8

	latchPC, latchIR, latchIRDesc, latchEXT bool
	latchOPA, latchOPB, latchADST, latchAR  bool
	latchDR, latchFlags, latchCnt, decCnt   bool
	rfWr, imemRd, dmemRd, dmemWr, halt      bool

	seq, selCond uint8
	condInv      bool
	addr         uint8
}

func decode(word uint64) signals {
	field := func(shift, width uint) uint8 { return uint8(word >> shift & (1<<width - 1)) } // вырезаем поле из слова
	flag := func(n uint) bool { return word>>n&1 != 0 }
	return signals{
		selRA:   field(shSelRA, 2),
		selAluA: field(shSelAluA, 2),
		selAluB: field(shSelAluB, 3),
		aluOp:   field(shAluOp, 4),

		selPC:    field(bitSelPC, 1),
		selWA:    field(bitSelWA, 1),
		selDAddr: field(bitSelDAddr, 1),

		latchPC:     flag(bitLatchPC),
		latchIR:     flag(bitLatchIR),
		latchIRDesc: flag(bitLatchIRDesc),
		latchEXT:    flag(bitLatchEXT),
		latchOPA:    flag(bitLatchOPA),
		latchOPB:    flag(bitLatchOPB),
		latchADST:   flag(bitLatchADST),
		latchAR:     flag(bitLatchAR),
		latchDR:     flag(bitLatchDR),
		latchFlags:  flag(bitLatchFlags),
		latchCnt:    flag(bitLatchCnt),
		decCnt:      flag(bitDecCnt),
		rfWr:        flag(bitRFWr),
		imemRd:      flag(bitIMemRd),
		dmemRd:      flag(bitDMemRd),
		dmemWr:      flag(bitDMemWr),
		halt:        flag(bitHalt),

		seq:     field(shSeq, 3),
		selCond: field(shSelCond, 2),
		condInv: flag(bitCondInv),
		addr:    field(shAddr, 8),
	}
}

type ControlUnit struct {
	MpmPC  uint8
	MpmRET uint8 // адрес возврата из микроподпрограммы
}

func (cu *ControlUnit) next(s signals, dp *DataPath) {
	// это Next address logic вместе с MUX_mpmPC, MUX_mpmRET и MUX_COND
	op := opMPM[dp.opcode()]
	switch s.seq {
	case seqNext:
		cu.MpmPC++
	case seqJmp:
		cu.MpmPC = s.addr
	case seqBr:
		condition := condBlock(dp.opcode()&0xF, dp.Flags) // cond_true
		if s.selCond == condCntZero {
			condition = dp.CNT == 0 // cnt_zero
		}
		if condition != s.condInv {
			cu.MpmPC = s.addr
		} else {
			cu.MpmPC++
		}
	case seqDecode:
		cu.MpmRET = op.addr
		cu.MpmPC = modeMPM1[op.wo][dp.m1()]
	case seqOp2:
		cu.MpmPC = modeMPM2[dp.m2()]
	case seqCall1:
		cu.MpmRET = s.addr
		cu.MpmPC = modeMPM1[op.wo][dp.m1()]
	case seqWB:
		cu.MpmRET = s.addr
		cu.MpmPC = wbMPM[dp.m1()]
	case seqRet:
		cu.MpmPC = cu.MpmRET
	}
}
