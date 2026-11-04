package machine

import (
	"fmt"
	"io"
	"strings"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
)

const (
	StopHalt           = "halt"
	StopInputExhausted = "input exhausted"
	StopTickLimit      = "tick limit exceeded"
	StopDivZero        = "error: division by zero"
	StopBadInstruction = "error: invalid instruction"
	StopBadAddress     = "error: unaligned or out of range memory address"
)

type Machine struct {
	DP DataPath
	CU ControlUnit

	imem, dmem []uint32 // память команд и память данных, по словам
	ICache     *Cache
	DCache     *Cache
	IO         *IOController

	Ticks        int
	WaitTicks    int // из них тактов ожидания памяти
	Instructions int
	Stopped      bool
	StopReason   string

	waiting bool // прошлый такт ждали кеш, значит FETCH повторяется и это не новая команда

	log      io.Writer // nil если журнал не нужен
	logLimit int       // 0 значит без ограничения
	logLines int
	logTail  []string // конец журнала, печатается после остановки
}

func New(prog isa.Program, input []byte, useCache bool) *Machine {
	m := &Machine{
		imem: make([]uint32, isa.MemSize/4),
		dmem: make([]uint32, isa.MemSize/4),
		IO:   &IOController{input: input},
	}
	copy(m.imem, prog.Code)
	copy(m.dmem, prog.Data)
	m.ICache = newCache(m.imem, useCache)
	m.DCache = newCache(m.dmem, useCache)
	m.DP.RF[isa.RegSP] = isa.MemSize // SP = 0x10000, первый push запишет в 0xFFFC
	return m
}

func (m *Machine) SetLog(w io.Writer, limit int) {
	m.log, m.logLimit = w, limit
}

func (m *Machine) Run(maxTicks int) {
	for !m.Stopped {
		if m.Ticks >= maxTicks {
			m.stop(StopTickLimit)
			break
		}
		m.Tick()
	}
	if skipped := m.logLines - m.logLimit; m.logLimit > 0 && skipped > 0 {
		m.writeLog(fmt.Sprintf("... %d lines skipped ...", skipped))
	}
	for _, line := range m.logTail { // конец журнала печатаем после остановки
		m.writeLog(line)
	}
	m.logTail = nil
}

func (m *Machine) stop(reason string) {
	m.Stopped, m.StopReason = true, reason
}

func validAddr(addr uint32) bool {
	return addr%4 == 0 && addr < isa.MemSize
}

func (m *Machine) Tick() {
	// один такт это одна микрокоманда из MPM
	dp := &m.DP
	mpc := m.CU.MpmPC
	s := decode(mpm[mpc])
	m.Ticks++
	if mpc == mFetch && !m.waiting {
		m.Instructions++
		m.logInstruction()
	}

	// комбинационная часть, мультиплексоры и АЛУ
	cond := condBlock(dp.opcode()&0xF, dp.Flags)
	a := dp.aluA(s.selAluA, s.selRA)
	b := dp.aluB(s.selAluB, cond)
	op := s.aluOp
	if op == aluOpcode { // операция из младших бит опкода
		op = dp.opcode() & 0xF
	}
	aluOut, flags, divZero := alu(op, a, b, dp.C)
	if divZero {
		m.stop(StopDivZero)
		m.logTick(mpc, "div_zero")
		return
	}

	// память, если кеш не готов, Stop logic выставляет stop
	var events []string
	var iWord, rdata uint32
	stop := false
	if s.imemRd {
		if !validAddr(dp.PC) {
			m.stop(StopBadAddress)
			m.logTick(mpc, "bad PC")
			return
		}
		ready, word, event := m.ICache.Access(dp.PC, false, 0)
		iWord, stop = word, !ready
		events = append(events, m.cacheEvent("I", m.ICache, event, dp.PC))
	}
	if (s.dmemRd || s.dmemWr) && !stop {
		address := dp.address(s.selDAddr)
		switch {
		case address == isa.PortIn || address == isa.PortOut:
			// MMIO Decoder отправляет порты в IO Controller мимо кеша
			if s.dmemRd && address == isa.PortIn {
				value, ok := m.IO.read()
				if !ok {
					m.stop(StopInputExhausted)
					m.logTick(mpc, "input exhausted")
					return
				}
				rdata = value
				events = append(events, "in "+isa.CharRepr(int32(value)))
			}
			if s.dmemWr && address == isa.PortOut {
				m.IO.write(aluOut)
				events = append(events, "out "+isa.CharRepr(int32(aluOut&0xFF)))
			}
		case !validAddr(address):
			m.stop(StopBadAddress)
			m.logTick(mpc, fmt.Sprintf("bad address %X", address))
			return
		default:
			ready, word, event := m.DCache.Access(address, s.dmemWr, aluOut)
			rdata, stop = word, !ready
			events = append(events, m.cacheEvent("D", m.DCache, event, address))
		}
	}
	m.waiting = stop
	if stop {
		m.WaitTicks++
		// stop блокирует защёлки и mpmPC, микрокоманда повторится в следующем такте
		m.logTick(mpc, strings.Join(events, ", "))
		return
	}

	// следующий адрес считаем по состоянию до защёлкивания
	m.CU.next(s, dp)

	// защёлкиваем регистры, все значения посчитаны по состоянию до такта, как в синхронной схеме
	old := *dp
	if s.latchPC {
		if s.selPC == pcAlu {
			dp.PC = aluOut
		} else {
			dp.PC = old.PC + 4
		}
	}
	if s.latchIR {
		dp.IR = iWord
	}
	if s.latchIRDesc {
		dp.IR = old.IR&^irDescMask | iWord&irDescMask // меняется только поле M1 R1
	}
	if s.latchEXT {
		dp.EXT = iWord
	}
	if s.latchOPA {
		dp.OPA = aluOut
	}
	if s.latchOPB {
		dp.OPB = aluOut
	}
	if s.latchADST {
		dp.ADST = aluOut
	}
	if s.latchAR {
		dp.AR = aluOut
	}
	if s.latchDR {
		dp.DR = rdata
	}
	if s.latchFlags {
		dp.Flags = flags
	}
	if s.latchCnt {
		dp.CNT = old.n()
	}
	if s.decCnt {
		dp.CNT = old.CNT - 1
	}
	if s.rfWr {
		if wa := old.writeAddr(s.selWA); wa < isa.NumRegs {
			dp.RF[wa] = aluOut
		}
	}

	if s.halt { // halt или ERROR
		if mpc == mExHalt {
			m.stop(StopHalt)
		} else {
			m.stop(StopBadInstruction)
		}
		events = append(events, "halt")
	}
	m.logTick(mpc, strings.Join(events, ", "))
}

func (m *Machine) cacheEvent(kind string, c *Cache, event string, addr uint32) string {
	if c.enabled {
		return fmt.Sprintf("%s-cache %s [%04X]", kind, event, addr)
	}
	return fmt.Sprintf("%sMem %s [%04X]", kind, event, addr) // без кеша идём прямо в память
}

func (m *Machine) logLine(format string, args ...any) {
	if m.log == nil {
		return
	}
	m.logLines++
	line := fmt.Sprintf(format, args...)
	head, tail := m.logLimit-m.logLimit/2, m.logLimit/2 // начало печатаем сразу, конец копим до остановки
	switch {
	case m.logLimit == 0 || m.logLines <= head:
		m.writeLog(line)
	case tail > 0:
		if len(m.logTail) == tail {
			m.logTail = m.logTail[1:]
		}
		m.logTail = append(m.logTail, line)
	}
}

func (m *Machine) writeLog(line string) {
	// журнал отладочный, ошибка записи в него не должна ломать моделирование
	_, _ = fmt.Fprintln(m.log, line)
}

func (m *Machine) logInstruction() {
	if m.log == nil || !validAddr(m.DP.PC) {
		return
	}
	text := "??"
	if in, _, err := isa.Decode(m.imem[m.DP.PC/4:]); err == nil {
		text = in.String()
	}
	m.logLine("instr %04X: %s", m.DP.PC, text)
}

func (m *Machine) logTick(mpc uint8, events string) {
	if m.log == nil {
		return
	}
	dp := &m.DP
	flag := func(b bool) byte {
		if b {
			return '1'
		}
		return '0'
	}
	line := fmt.Sprintf("%6d | %2d %-9s | PC=%04X IR=%08X EXT=%X OPA=%X OPB=%X ADST=%X AR=%X DR=%X CNT=%d |",
		m.Ticks, mpc, microLabel(mpc), dp.PC, dp.IR, dp.EXT, dp.OPA, dp.OPB, dp.ADST, dp.AR, dp.DR, dp.CNT)
	for r := range uint8(isa.NumRegs) {
		line += fmt.Sprintf(" %s=%X", isa.RegName(r), dp.RF[r])
	}
	line += fmt.Sprintf(" | NZVC=%c%c%c%c", flag(dp.N), flag(dp.Z), flag(dp.V), flag(dp.C))
	if events != "" {
		line += " | " + events
	}
	m.logLine("%s", line)
}
