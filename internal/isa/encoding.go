package isa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

func firstWord(op Opcode, m1 Mode, r1 uint8, m2 Mode, r2, n uint8) uint32 {
	// 31..24 opcode, 23..20 M1, 19..16 R1, 15..12 M2, 11..8 R2, 7..0 N
	return uint32(op)<<24 | uint32(m1&0xF)<<20 | uint32(r1&0xF)<<16 |
		uint32(m2&0xF)<<12 | uint32(r2&0xF)<<8 | uint32(n)
}

func descWord(o Operand) uint32 {
	// режим и регистр в тех же битах, что M1 и R1 в первом слове
	return uint32(o.Mode&0xF)<<20 | uint32(o.Reg&0xF)<<16
}

func (in Instr) Encode() []uint32 {
	words := make([]uint32, 0, in.Size())
	if in.Op.Arity() == ArityVar { // push и pop, в первом слове только opcode и N
		words = append(words, firstWord(in.Op, ModeNone, 0, ModeNone, 0, uint8(len(in.Operands))))
		for _, o := range in.Operands {
			words = append(words, descWord(o))
			if o.Mode.HasExt() {
				words = append(words, uint32(o.Value))
			}
		}
		return words
	}

	var a, b Operand
	if len(in.Operands) > 0 {
		a = in.Operands[0]
	}
	if len(in.Operands) > 1 {
		b = in.Operands[1]
	}
	words = append(words, firstWord(in.Op, a.Mode, a.Reg, b.Mode, b.Reg, 0))
	if a.Mode.HasExt() {
		words = append(words, uint32(a.Value))
	}
	if b.Mode.HasExt() {
		words = append(words, uint32(b.Value))
	}
	return words
}

var ErrDecode = errors.New("invalid instruction")

func Decode(words []uint32) (Instr, int, error) {
	if len(words) == 0 {
		return Instr{}, 0, ErrDecode
	}
	w := words[0]
	in := Instr{Op: Opcode(w >> 24)}
	if !in.Op.Valid() {
		return in, 1, fmt.Errorf("%w: opcode 0x%02X", ErrDecode, w>>24)
	}
	pos := 1
	// режим и регистр всегда лежат в битах 23..16
	operand := func(field uint32) (Operand, error) {
		o := Operand{Mode: Mode(field>>20) & 0xF, Reg: uint8(field>>16) & 0xF}
		if o.Mode > ModeOff {
			return o, fmt.Errorf("%w: addressing mode %d", ErrDecode, o.Mode)
		}
		if o.Mode.HasExt() {
			if pos >= len(words) {
				return o, fmt.Errorf("%w: missing extension word", ErrDecode)
			}
			o.Value = int32(words[pos])
			pos++
		}
		return o, nil
	}

	switch in.Op.Arity() {
	case ArityVar:
		for range int(w & 0xFF) {
			if pos >= len(words) {
				return in, pos, fmt.Errorf("%w: missing operand descriptor", ErrDecode)
			}
			pos++
			o, err := operand(words[pos-1])
			if err != nil {
				return in, pos, err
			}
			in.Operands = append(in.Operands, o)
		}
	case Arity1, Arity2:
		o, err := operand(w)
		if err != nil {
			return in, pos, err
		}
		in.Operands = append(in.Operands, o)
		if in.Op.Arity() == Arity2 {
			o, err = operand(w << 8) // поле M2 R2 сдвигается на место M1 R1
			if err != nil {
				return in, pos, err
			}
			in.Operands = append(in.Operands, o)
		}
	case Arity0:
	}
	return in, pos, nil
}

type Program struct {
	Code []uint32 // оба образа грузятся в свою память с адреса 0
	Data []uint32
}

const magic = "LAB4"

func (p *Program) MarshalBinary() ([]byte, error) {
	// LAB4, размеры кода и данных в байтах, потом сами образы, всё little-endian
	var buf bytes.Buffer
	buf.WriteString(magic)
	header := []uint32{uint32(4 * len(p.Code)), uint32(4 * len(p.Data))}
	for _, part := range [][]uint32{header, p.Code, p.Data} {
		if err := binary.Write(&buf, binary.LittleEndian, part); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

var ErrFormat = errors.New("invalid binary format")

func (p *Program) UnmarshalBinary(b []byte) error {
	if len(b) < 12 || string(b[:4]) != magic {
		return fmt.Errorf("%w: no LAB4 signature", ErrFormat)
	}
	codeSize := binary.LittleEndian.Uint32(b[4:8])
	dataSize := binary.LittleEndian.Uint32(b[8:12])
	if codeSize%4 != 0 || dataSize%4 != 0 || codeSize > MemSize || dataSize > MemSize ||
		len(b) != 12+int(codeSize)+int(dataSize) {
		return fmt.Errorf("%w: bad section sizes", ErrFormat)
	}
	words := func(raw []byte) []uint32 {
		out := make([]uint32, len(raw)/4)
		for i := range out {
			out[i] = binary.LittleEndian.Uint32(raw[4*i:])
		}
		return out
	}
	p.Code = words(b[12 : 12+codeSize])
	p.Data = words(b[12+codeSize:])
	return nil
}
