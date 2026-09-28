package metrics

import (
	"errors"
	"fmt"
)

var ErrParse = errors.New("formula parse error")

type parser struct {
	tokens []token
	pos    int
}

func Parse(input string) (Node, error) {
	input = normalizeFormula(input)
	if input == "" {
		return Node{}, fmt.Errorf("%w: empty formula", ErrParse)
	}

	tokens, err := lex(input)
	if err != nil {
		return Node{}, fmt.Errorf("%w: %v", ErrParse, err)
	}

	p := &parser{tokens: tokens}
	node, err := p.parseExpr()
	if err != nil {
		return Node{}, err
	}

	if p.peek().kind != tokEOF {
		return Node{}, fmt.Errorf("%w: unexpected token %q at position %d", ErrParse, p.peek().text, p.peek().pos)
	}

	return node, nil
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

func (p *parser) parseExpr() (Node, error) {
	return p.parseAddSub()
}

func (p *parser) parseAddSub() (Node, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return Node{}, err
	}

	for p.peek().kind == tokOp && (p.peek().text == "+" || p.peek().text == "-") {
		op := p.advance().text
		right, err := p.parseMulDiv()
		if err != nil {
			return Node{}, err
		}
		left = Node{
			Type:     NodeBinary,
			Op:       op,
			Children: []Node{left, right},
		}
	}

	return left, nil
}

func (p *parser) parseMulDiv() (Node, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return Node{}, err
	}

	for p.peek().kind == tokOp && (p.peek().text == "*" || p.peek().text == "/") {
		op := p.advance().text
		right, err := p.parsePrimary()
		if err != nil {
			return Node{}, err
		}
		left = Node{
			Type:     NodeBinary,
			Op:       op,
			Children: []Node{left, right},
		}
	}

	return left, nil
}

func (p *parser) parsePrimary() (Node, error) {
	t := p.peek()

	switch t.kind {
	case tokNumber:
		p.advance()
		return Node{Type: NodeNumber, Value: t.value}, nil

	case tokIdent:
		p.advance()
		if p.peek().kind == tokLParen {
			return p.parseFuncCall(t.text)
		}
		return Node{Type: NodeField, Field: t.text}, nil

	case tokLParen:
		p.advance()
		node, err := p.parseExpr()
		if err != nil {
			return Node{}, err
		}
		if p.peek().kind != tokRParen {
			return Node{}, fmt.Errorf("%w: expected ) at position %d", ErrParse, p.peek().pos)
		}
		p.advance()
		return node, nil

	case tokOp:
		if t.text == "-" {
			p.advance()
			operand, err := p.parsePrimary()
			if err != nil {
				return Node{}, err
			}
			return Node{
				Type:     NodeBinary,
				Op:       "*",
				Children: []Node{{Type: NodeNumber, Value: -1}, operand},
			}, nil
		}
		return Node{}, fmt.Errorf("%w: unexpected operator %q at position %d", ErrParse, t.text, t.pos)

	default:
		return Node{}, fmt.Errorf("%w: unexpected token %q at position %d", ErrParse, t.text, t.pos)
	}
}

func (p *parser) parseFuncCall(name string) (Node, error) {
	p.advance()

	var children []Node

	if p.peek().kind != tokRParen {
		for {
			arg, err := p.parseExpr()
			if err != nil {
				return Node{}, err
			}
			children = append(children, arg)

			if p.peek().kind == tokComma {
				p.advance()
				continue
			}
			break
		}
	}

	if p.peek().kind != tokRParen {
		return Node{}, fmt.Errorf("%w: expected ) at position %d", ErrParse, p.peek().pos)
	}
	p.advance()

	return Node{
		Type:     NodeFunc,
		Name:     name,
		Children: children,
	}, nil
}
