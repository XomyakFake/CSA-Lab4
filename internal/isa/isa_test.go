package isa

import (
	"reflect"
	"testing"
)

func TestEncodeDocExamples(t *testing.T) {
	cases := []struct {
		in   Instr
		text string
		want []uint32
	}{
		// примеры из docs/memory-isa.md
		{Instr{OpAdd, []Operand{Reg(1), Reg(2)}}, "add R1, R2", []uint32{0x10111200}},
		{Instr{OpAdd, []Operand{Reg(1), Imm(5)}}, "add R1, #5", []uint32{0x10112000, 5}},
		{Instr{OpMov, []Operand{Abs(0x84), Imm(104)}}, "mov [0x0084], #104", []uint32{0x01302000, 0x84, 0x68}},
		{
			Instr{OpPush, []Operand{Reg(1), Imm(5), Off(RegFP, -8)}},
			"push R1, #5, [FP-8]",
			[]uint32{0x02000003, 0x00110000, 0x00200000, 5, 0x00580000, 0xFFFFFFF8},
		},
		{Instr{OpAdd, []Operand{Off(RegFP, -4), Imm(1)}}, "add [FP-4], #1", []uint32{0x10582000, 0xFFFFFFFC, 1}},
		{Instr{OpCmp, []Operand{Off(RegFP, 8), Imm(0)}}, "cmp [FP+8], #0", []uint32{0x19582000, 8, 0}},
		{Instr{OpJmp, []Operand{Imm(0x1C)}}, "jmp #0x001C", []uint32{0x20200000, 0x1C}},
		{Instr{OpPop, []Operand{Reg(RegFP)}}, "pop FP", []uint32{0x03000001, 0x00180000}},
		{Instr{Op: OpHalt}, "halt", []uint32{0}},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			got := c.in.Encode()
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("encode: got %08X, want %08X", got, c.want)
			}
			if len(got) != c.in.Size() {
				t.Fatalf("size: got %d, want %d", c.in.Size(), len(got))
			}
			dec, n, err := Decode(got)
			if err != nil || n != len(got) {
				t.Fatalf("decode: n=%d err=%v", n, err)
			}
			if dec.String() != c.text {
				t.Fatalf("text: got %q, want %q", dec.String(), c.text)
			}
		})
	}
}

func TestBinaryRoundTrip(t *testing.T) {
	p := Program{Code: []uint32{0x01302000, 0x84, 0x68, 0}, Data: []uint32{1, 2, 3}}
	raw, err := p.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw[:4]) != "LAB4" || len(raw) != 12+4*7 {
		t.Fatalf("bad header: % X", raw[:12])
	}
	var q Program
	if err := q.UnmarshalBinary(raw); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, q) {
		t.Fatalf("round trip: got %v, want %v", q, p)
	}
	if err := q.UnmarshalBinary(raw[:len(raw)-1]); err == nil {
		t.Fatal("truncated file must be rejected")
	}
}

func TestListing(t *testing.T) {
	code := append(Instr{OpMov, []Operand{Abs(PortOut), Imm('h')}}.Encode(), 0)
	got := CodeListing(code, nil)
	want := "0000 - 01302000 00000084 00000068 - mov [0x0084], #104\n000C - 00000000 - halt\n"
	if got != want {
		t.Fatalf("code listing:\n%s\nwant:\n%s", got, want)
	}
	data := make([]uint32, StaticBase/4, StaticBase/4+2)
	data = append(data, 'h', 0)
	got = DataListing(data, map[int]string{0x88: CharRepr('h'), 0x8C: CharRepr(0)})
	want = "0088 - 00000068 - 'h'\n008C - 00000000 - '\\0'\n"
	if got != want {
		t.Fatalf("data listing:\n%s\nwant:\n%s", got, want)
	}
}
