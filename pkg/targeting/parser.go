package targeting

import (
	"fmt"
)

type parser struct {
	tokens []token
	pos    int
}

// Parse parses a targeting DSL expression into a canonical AST.
func Parse(input string) (Node, error) {
	tokens, err := lex(input)
	if err != nil {
		return Node{}, err
	}
	p := &parser{tokens: tokens}
	node, err := p.parseOr()
	if err != nil {
		return Node{}, err
	}
	if p.peek().kind != tokEOF {
		return Node{}, fmt.Errorf("%w: unexpected token %q at %d", ErrSyntax, p.peek().text, p.peek().pos)
	}
	return node, nil
}

// Canonicalize parses a DSL expression and returns its canonical JSON encoding.
func Canonicalize(input string) ([]byte, error) {
	node, err := Parse(input)
	if err != nil {
		return nil, err
	}
	return marshalNode(node)
}

// IsEmpty reports whether raw is absent/blank/JSON null/empty object.
func IsEmpty(raw []byte) bool {
	trimmed := trimSpaceBytes(raw)
	if len(trimmed) == 0 {
		return true
	}
	switch string(trimmed) {
	case "null", "{}":
		return true
	default:
		return false
	}
}

// CanonicalJSON converts persisted targeting bytes (DSL string or canonical
// AST JSON) into canonical AST JSON. Empty input returns (nil, nil).
func CanonicalJSON(raw []byte) ([]byte, error) {
	if IsEmpty(raw) {
		return nil, nil
	}
	trimmed := trimSpaceBytes(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		if _, err := unmarshalNode(trimmed); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidAST, err)
		}
		return trimmed, nil
	}
	return Canonicalize(string(trimmed))
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return Node{}, err
	}
	for p.peek().kind == tokOr {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return Node{}, err
		}
		left = Node{Type: NodeOr, Children: []Node{left, right}}
	}
	return left, nil
}

func (p *parser) parseAnd() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return Node{}, err
	}
	for p.peek().kind == tokAnd {
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return Node{}, err
		}
		left = Node{Type: NodeAnd, Children: []Node{left, right}}
	}
	return left, nil
}

func (p *parser) parseUnary() (Node, error) {
	if p.peek().kind == tokNot {
		p.advance()
		child, err := p.parseUnary()
		if err != nil {
			return Node{}, err
		}
		return Node{Type: NodeNot, Child: &child}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Node, error) {
	tok := p.peek()
	if tok.kind == tokLParen {
		p.advance()
		node, err := p.parseOr()
		if err != nil {
			return Node{}, err
		}
		if p.peek().kind != tokRParen {
			return Node{}, fmt.Errorf("%w: expected ')' at %d", ErrSyntax, p.peek().pos)
		}
		p.advance()
		return node, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (Node, error) {
	fieldTok := p.peek()
	if fieldTok.kind != tokIdent {
		return Node{}, fmt.Errorf("%w: expected field at %d", ErrSyntax, fieldTok.pos)
	}
	p.advance()
	field := fieldTok.text

	opTok := p.peek()
	switch opTok.kind {
	case tokOp:
		p.advance()
		value, err := p.parseLiteral()
		if err != nil {
			return Node{}, err
		}
		return Node{Type: NodeCmp, Field: field, Operator: opTok.text, Value: value}, nil
	case tokIn:
		p.advance()
		values, err := p.parseArray()
		if err != nil {
			return Node{}, err
		}
		return Node{Type: NodeIn, Field: field, Values: values, Negated: asBool(opTok.value)}, nil
	case tokNot:
		p.advance()
		if p.peek().kind != tokIn {
			return Node{}, fmt.Errorf("%w: expected IN after NOT at %d", ErrSyntax, p.peek().pos)
		}
		p.advance()
		values, err := p.parseArray()
		if err != nil {
			return Node{}, err
		}
		return Node{Type: NodeIn, Field: field, Values: values, Negated: true}, nil
	default:
		return Node{}, fmt.Errorf("%w: expected operator after %q at %d", ErrSyntax, field, opTok.pos)
	}
}

func (p *parser) parseLiteral() (any, error) {
	tok := p.peek()
	switch tok.kind {
	case tokString, tokNumber, tokBool:
		p.advance()
		return tok.value, nil
	case tokNull:
		p.advance()
		return nil, nil
	case tokLBracket:
		values, err := p.parseArray()
		if err != nil {
			return nil, err
		}
		return values, nil
	default:
		return nil, fmt.Errorf("%w: expected literal at %d", ErrSyntax, tok.pos)
	}
}

func (p *parser) parseArray() ([]any, error) {
	if p.peek().kind != tokLBracket {
		return nil, fmt.Errorf("%w: expected '[' at %d", ErrSyntax, p.peek().pos)
	}
	p.advance()
	values := make([]any, 0, 4)
	if p.peek().kind == tokRBracket {
		p.advance()
		return values, nil
	}
	for {
		value, err := p.parseLiteral()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		switch p.peek().kind {
		case tokComma:
			p.advance()
		case tokRBracket:
			p.advance()
			return values, nil
		default:
			return nil, fmt.Errorf("%w: expected ',' or ']' at %d", ErrSyntax, p.peek().pos)
		}
	}
}

func asBool(v any) bool {
	b, ok := v.(bool)
	return ok && b
}
