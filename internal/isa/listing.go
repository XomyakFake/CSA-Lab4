package isa

import (
	"fmt"
	"strconv"
	"strings"
)

func CodeListing(code []uint32, labels map[int]string) string {
	var sb strings.Builder
	for pc := 0; pc < len(code); {
		if name, ok := labels[4*pc]; ok {
			fmt.Fprintf(&sb, "; %s\n", name) // имя функции перед её первой командой
		}
		in, n, err := Decode(code[pc:])
		text := in.String()
		if err != nil {
			n, text = 1, "?? "+err.Error()
		}
		hex := make([]string, n)
		for i := range n {
			hex[i] = fmt.Sprintf("%08X", code[pc+i])
		}
		fmt.Fprintf(&sb, "%04X - %s - %s\n", 4*pc, strings.Join(hex, " "), text)
		pc += n
	}
	return sb.String()
}

func DataListing(data []uint32, notes map[int]string) string {
	var sb strings.Builder
	for i := StaticBase / 4; i < len(data); i++ { // порты и пустое начало памяти не печатаем
		fmt.Fprintf(&sb, "%04X - %08X - %s\n", 4*i, data[i], notes[4*i])
	}
	return sb.String()
}

func CharRepr(c int32) string {
	switch c {
	case 0:
		return `'\0'`
	case '\n':
		return `'\n'`
	case '\t':
		return `'\t'`
	case '\\':
		return `'\\'`
	case '\'':
		return `'\''`
	}
	if c >= 0x20 && c < 0x7F {
		return "'" + string(c) + "'"
	}
	return strconv.Itoa(int(c))
}
