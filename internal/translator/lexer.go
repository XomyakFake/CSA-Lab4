package translator

import (
	"fmt"
	"strconv"
	"strings"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber // число или символ, значение в value
	tokString // текст строки в text
	tokKeyword
	tokPunct // знак операции, сам знак в text
)

type token struct {
	kind   tokenKind
	text   string
	value  int32
	isChar bool // число было записано как 'c', нужно для дампа AST
	line   int
}

var keywords = map[string]bool{
	"auto": true, "const": true, "func": true, "if": true,
	"else": true, "while": true, "for": true, "return": true,
}

var punct2 = []string{"||", "&&", "==", "!=", "<=", ">="} // проверяем раньше односимвольных

const punct1 = "+-*/%&|!~<>=(){}[],;"

type translateError struct {
	line int
	msg  string
}

func (e translateError) Error() string {
	return fmt.Sprintf("line %d: %s", e.line, e.msg)
}

func failf(line int, format string, args ...any) {
	panic(translateError{line, fmt.Sprintf(format, args...)}) // Translate поймает через recover
}

type lexer struct {
	src  string
	pos  int
	line int
}

func tokenize(src string) []token {
	lx := lexer{src: src, line: 1}
	var tokens []token
	for {
		t := lx.next()
		tokens = append(tokens, t)
		if t.kind == tokEOF {
			return tokens
		}
	}
}

func (lx *lexer) skipSpaceAndComments() {
	for lx.pos < len(lx.src) {
		c := lx.src[lx.pos]
		switch {
		case c == '\n':
			lx.line++
			lx.pos++
		case c == ' ' || c == '\t' || c == '\r':
			lx.pos++
		case strings.HasPrefix(lx.src[lx.pos:], "//"):
			for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' {
				lx.pos++
			}
		default:
			return
		}
	}
}

func isLetter(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func (lx *lexer) next() token {
	lx.skipSpaceAndComments()
	if lx.pos >= len(lx.src) {
		return token{kind: tokEOF, line: lx.line}
	}
	c := lx.src[lx.pos]
	switch {
	case isLetter(c):
		start := lx.pos
		for lx.pos < len(lx.src) && (isLetter(lx.src[lx.pos]) || isDigit(lx.src[lx.pos])) {
			lx.pos++
		}
		word := lx.src[start:lx.pos]
		if keywords[word] {
			return token{kind: tokKeyword, text: word, line: lx.line}
		}
		return token{kind: tokIdent, text: word, line: lx.line}
	case isDigit(c):
		return lx.number()
	case c == '\'':
		lx.pos++
		v := lx.charInLiteral('\'')
		if lx.pos >= len(lx.src) || lx.src[lx.pos] != '\'' {
			failf(lx.line, "unterminated character literal")
		}
		lx.pos++
		return token{kind: tokNumber, value: int32(v), isChar: true, line: lx.line}
	case c == '"':
		lx.pos++
		var sb strings.Builder
		for lx.pos < len(lx.src) && lx.src[lx.pos] != '"' {
			sb.WriteByte(lx.charInLiteral('"'))
		}
		if lx.pos >= len(lx.src) {
			failf(lx.line, "unterminated string literal")
		}
		lx.pos++
		return token{kind: tokString, text: sb.String(), line: lx.line}
	}
	for _, p := range punct2 {
		if strings.HasPrefix(lx.src[lx.pos:], p) {
			lx.pos += 2
			return token{kind: tokPunct, text: p, line: lx.line}
		}
	}
	if strings.IndexByte(punct1, c) >= 0 {
		lx.pos++
		return token{kind: tokPunct, text: string(c), line: lx.line}
	}
	failf(lx.line, "unexpected character %q", c)
	return token{}
}

func (lx *lexer) number() token {
	start := lx.pos
	base := 10
	if strings.HasPrefix(lx.src[lx.pos:], "0x") {
		base = 16
		lx.pos += 2
		start = lx.pos
	}
	for lx.pos < len(lx.src) && (isDigit(lx.src[lx.pos]) || isLetter(lx.src[lx.pos])) {
		lx.pos++
	}
	v, err := strconv.ParseUint(lx.src[start:lx.pos], base, 32) // 0xFFFFFFFF тоже можно, это -1
	if err != nil {
		failf(lx.line, "bad number %q", lx.src[start:lx.pos])
	}
	return token{kind: tokNumber, value: int32(uint32(v)), line: lx.line}
}

var escapes = map[byte]byte{'n': '\n', 't': '\t', '0': 0, '\\': '\\', '\'': '\'', '"': '"'}

func (lx *lexer) charInLiteral(quote byte) byte {
	if lx.pos >= len(lx.src) {
		failf(lx.line, "unterminated literal")
	}
	c := lx.src[lx.pos]
	lx.pos++
	switch {
	case c == '\\':
		if lx.pos >= len(lx.src) {
			failf(lx.line, "unterminated literal")
		}
		e, ok := escapes[lx.src[lx.pos]]
		if !ok {
			failf(lx.line, "unknown escape sequence \\%c", lx.src[lx.pos])
		}
		lx.pos++
		return e
	case c < 0x20 || c > 0x7E || c == quote:
		failf(lx.line, "bad character in literal")
	}
	return c
}
