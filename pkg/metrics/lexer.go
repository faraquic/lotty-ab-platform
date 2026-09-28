package metrics

import (
	"fmt"
	"strings"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber
	tokOp
	tokLParen
	tokRParen
	tokComma
)

type token struct {
	kind  tokenKind
	text  string
	value float64
	pos   int
}

type lexer struct {
	input string
	pos   int
}

func lex(input string) ([]token, error) {
	l := &lexer{input: input}
	var tokens []token

	for {
		t, err := l.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
		if t.kind == tokEOF {
			break
		}
	}

	return tokens, nil
}

func (l *lexer) next() (token, error) {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return token{kind: tokEOF, pos: l.pos}, nil
	}

	ch := l.input[l.pos]

	switch {
	case ch == '(':
		l.pos++
		return token{kind: tokLParen, text: "(", pos: l.pos - 1}, nil
	case ch == ')':
		l.pos++
		return token{kind: tokRParen, text: ")", pos: l.pos - 1}, nil
	case ch == ',':
		l.pos++
		return token{kind: tokComma, text: ",", pos: l.pos - 1}, nil
	case ch == '+' || ch == '-' || ch == '*' || ch == '/':
		op := string(ch)
		l.pos++
		return token{kind: tokOp, text: op, pos: l.pos - 1}, nil
	case ch >= '0' && ch <= '9' || ch == '.':
		return l.lexNumber()
	case isIdentStart(ch):
		return l.lexIdent()
	default:
		return token{}, fmt.Errorf("unexpected character %q at position %d", ch, l.pos)
	}
}

func (l *lexer) lexNumber() (token, error) {
	start := l.pos
	hasDot := false

	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch >= '0' && ch <= '9' {
			l.pos++
		} else if ch == '.' && !hasDot {
			hasDot = true
			l.pos++
		} else {
			break
		}
	}

	text := l.input[start:l.pos]
	var value float64
	if _, err := fmt.Sscanf(text, "%g", &value); err != nil {
		return token{}, fmt.Errorf("invalid number %q at position %d", text, start)
	}

	return token{kind: tokNumber, text: text, value: value, pos: start}, nil
}

func (l *lexer) lexIdent() (token, error) {
	start := l.pos

	for l.pos < len(l.input) && isIdentPart(l.input[l.pos]) {
		l.pos++
	}

	return token{kind: tokIdent, text: l.input[start:l.pos], pos: start}, nil
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t' || l.input[l.pos] == '\n' || l.input[l.pos] == '\r') {
		l.pos++
	}
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func normalizeFormula(input string) string {
	return strings.TrimSpace(input)
}
