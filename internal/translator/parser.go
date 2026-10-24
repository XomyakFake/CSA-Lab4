package translator

type parser struct {
	tokens []token
	pos    int
}

func parse(src string) *program {
	// рекурсивный спуск по грамматике из docs/language.md, на каждое правило свой метод
	p := parser{tokens: tokenize(src)}
	prog := &program{}
	for p.peek().kind != tokEOF {
		prog.items = append(prog.items, p.topItem())
	}
	return prog
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.tokens[p.pos]
	if t.kind != tokEOF {
		p.pos++
	}
	return t
}

func (p *parser) is(text string) bool {
	t := p.peek()
	return (t.kind == tokKeyword || t.kind == tokPunct) && t.text == text
}

func (p *parser) accept(text string) bool {
	if p.is(text) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) expect(text string) {
	if !p.is(text) {
		failf(p.peek().line, "expected %q, got %q", text, p.describe())
	}
	p.advance()
}

func (p *parser) describe() string {
	t := p.peek()
	switch t.kind {
	case tokEOF:
		return "end of file"
	case tokNumber:
		return "number"
	case tokString:
		return "string"
	default:
		return t.text
	}
}

func (p *parser) identName() (name string, line int) {
	t := p.peek()
	if t.kind != tokIdent {
		failf(t.line, "expected identifier, got %q", p.describe())
	}
	p.pos++
	return t.text, t.line
}

func (p *parser) topItem() node {
	switch {
	case p.is("const"):
		line := p.advance().line
		name, _ := p.identName()
		p.expect("=")
		value := p.expression()
		p.expect(";")
		return &constDecl{name: name, value: value, line: line}
	case p.is("func"):
		line := p.advance().line
		name, _ := p.identName()
		p.expect("(")
		var params []string
		if !p.is(")") {
			for {
				p.expect("auto")
				param, _ := p.identName()
				params = append(params, param)
				if !p.accept(",") {
					break
				}
			}
		}
		p.expect(")")
		return &funcDecl{name: name, params: params, body: p.block(), line: line}
	default:
		return p.statement()
	}
}

func (p *parser) block() *block {
	p.expect("{")
	b := &block{}
	for !p.is("}") {
		if p.peek().kind == tokEOF {
			failf(p.peek().line, "expected \"}\", got end of file")
		}
		b.stmts = append(b.stmts, p.statement())
	}
	p.expect("}")
	return b
}

func (p *parser) statement() node {
	switch {
	case p.is("auto"):
		decl := p.varDecl()
		p.expect(";")
		return decl
	case p.is("if"):
		return p.ifStmt()
	case p.is("while"):
		p.advance()
		p.expect("(")
		cond := p.expression()
		p.expect(")")
		return &whileStmt{cond: cond, body: p.block()}
	case p.is("for"):
		return p.forStmt()
	case p.is("return"):
		ret := &returnStmt{line: p.advance().line}
		if !p.is(";") {
			ret.value = p.expression()
		}
		p.expect(";")
		return ret
	case p.is("{"):
		return p.block()
	default:
		expr := p.expression()
		p.expect(";")
		return expr
	}
}

func (p *parser) varDecl() *varDecl {
	p.expect("auto")
	name, line := p.identName()
	decl := &varDecl{name: name, line: line}
	switch {
	case p.accept("="):
		decl.init = p.expression()
	case p.accept("["):
		decl.size = p.expression()
		p.expect("]")
	}
	return decl
}

func (p *parser) ifStmt() *ifStmt {
	p.expect("if")
	p.expect("(")
	cond := p.expression()
	p.expect(")")
	stmt := &ifStmt{cond: cond, then: p.block()}
	if p.accept("else") {
		if p.is("if") {
			stmt.els = p.ifStmt()
		} else {
			stmt.els = p.block()
		}
	}
	return stmt
}

func (p *parser) forStmt() *forStmt {
	p.expect("for")
	p.expect("(")
	stmt := &forStmt{}
	if !p.is(";") {
		if p.is("auto") {
			stmt.init = p.varDecl()
		} else {
			stmt.init = p.expression()
		}
	}
	p.expect(";")
	if !p.is(";") {
		stmt.cond = p.expression()
	}
	p.expect(";")
	if !p.is(")") {
		stmt.step = p.expression()
	}
	p.expect(")")
	stmt.body = p.block()
	return stmt
}

func (p *parser) expression() node {
	// по одному токену не понять, присваивание это или нет, поэтому
	// сначала разбираем выражение, а при "=" проверяем что слева x или a[i]
	left := p.binaryLevel(0)
	if !p.is("=") {
		return left
	}
	line := p.advance().line
	switch left.(type) {
	case *ident, *index:
	default:
		failf(line, "left side of assignment must be a variable or an indexed element")
	}
	return &assign{target: left, value: p.expression(), line: line} // рекурсия даёт правую ассоциативность
}

var binaryLevels = [][]string{ // от самого слабого оператора к самому сильному
	{"||"},
	{"&&"},
	{"|"},
	{"&"},
	{"==", "!="},
	{"<", "<=", ">", ">="},
	{"+", "-"},
	{"*", "/", "%"},
}

func (p *parser) binaryLevel(level int) node {
	// все уровни от || до * устроены одинаково, поэтому один метод на всех
	if level == len(binaryLevels) {
		return p.unary()
	}
	left := p.binaryLevel(level + 1)
	for {
		op := ""
		for _, candidate := range binaryLevels[level] {
			if p.peek().kind == tokPunct && p.peek().text == candidate {
				op = candidate
			}
		}
		if op == "" {
			return left
		}
		p.advance()
		left = &binary{op: op, l: left, r: p.binaryLevel(level + 1)}
	}
}

func (p *parser) unary() node {
	for _, op := range []string{"-", "!", "~"} {
		if p.peek().kind == tokPunct && p.peek().text == op {
			p.advance()
			return &unary{op: op, x: p.unary()}
		}
	}
	return p.postfix()
}

func (p *parser) postfix() node {
	expr := p.primary()
	for p.accept("[") {
		expr = &index{base: expr, idx: p.expression()}
		p.expect("]")
	}
	return expr
}

func (p *parser) primary() node {
	t := p.peek()
	switch t.kind {
	case tokNumber:
		p.advance()
		return &numLit{value: t.value, isChar: t.isChar}
	case tokString:
		p.advance()
		return &strLit{value: t.text}
	case tokIdent:
		p.advance()
		if !p.accept("(") {
			return &ident{name: t.text, line: t.line}
		}
		c := &call{name: t.text, line: t.line}
		if !p.is(")") {
			for {
				c.args = append(c.args, p.expression())
				if !p.accept(",") {
					break
				}
			}
		}
		p.expect(")")
		return c
	default:
		if p.accept("(") {
			expr := p.expression()
			p.expect(")")
			return expr
		}
		failf(t.line, "expected expression, got %q", p.describe())
		return nil
	}
}
