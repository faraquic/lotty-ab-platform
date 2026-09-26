package targeting

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/goccy/go-json"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokString
	tokNumber
	tokBool
	tokNull
	tokLParen
	tokRParen
	tokLBracket
	tokRBracket
	tokComma
	tokOp
	tokAnd
	tokOr
	tokNot
	tokIn
)

type token struct {
	kind  tokenKind
	text  string
	value any
	pos   int
}

type lexer struct {
	input string
	pos   int
}

func lex(input string) ([]token, error) {
	l := &lexer{input: input}
	var out []token
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		out = append(out, tok)
		if tok.kind == tokEOF {
			return out, nil
		}
	}
}

func (l *lexer) next() (token, error) {
	l.skipSpace()
	if l.pos >= len(l.input) {
		return token{kind: tokEOF, pos: l.pos}, nil
	}
	start := l.pos
	c := l.input[l.pos]

	switch {
	case c == '(':
		l.pos++
		return token{kind: tokLParen, text: "(", pos: start}, nil
	case c == ')':
		l.pos++
		return token{kind: tokRParen, text: ")", pos: start}, nil
	case c == '[':
		l.pos++
		return token{kind: tokLBracket, text: "[", pos: start}, nil
	case c == ']':
		l.pos++
		return token{kind: tokRBracket, text: "]", pos: start}, nil
	case c == ',':
		l.pos++
		return token{kind: tokComma, text: ",", pos: start}, nil
	case c == '"' || c == '\'':
		return l.lexString()
	case c == '=' || c == '!' || c == '>' || c == '<':
		return l.lexOperator()
	case c == '-' || (c >= '0' && c <= '9'):
		return l.lexNumber()
	case isIdentStart(rune(c)):
		return l.lexIdent()
	default:
		return token{}, fmt.Errorf("%w: unexpected character %q at %d", ErrSyntax, string(c), start)
	}
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.input) {
		r, size := utf8.DecodeRuneInString(l.input[l.pos:])
		if !unicode.IsSpace(r) {
			return
		}
		l.pos += size
	}
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_' || r == '.' || r == '$'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '$' || r == '-'
}

func (l *lexer) lexIdent() (token, error) {
	start := l.pos
	for l.pos < len(l.input) {
		r, size := utf8.DecodeRuneInString(l.input[l.pos:])
		if !isIdentPart(r) {
			break
		}
		l.pos += size
	}
	text := l.input[start:l.pos]
	switch strings.ToLower(text) {
	case "and":
		return token{kind: tokAnd, text: text, pos: start}, nil
	case "or":
		return token{kind: tokOr, text: text, pos: start}, nil
	case "notin":
		return token{kind: tokIn, text: text, value: true, pos: start}, nil
	case "not":
		return token{kind: tokNot, text: text, pos: start}, nil
	case "in":
		return token{kind: tokIn, text: text, value: false, pos: start}, nil
	case "true":
		return token{kind: tokBool, text: text, value: true, pos: start}, nil
	case "false":
		return token{kind: tokBool, text: text, value: false, pos: start}, nil
	case "null", "nil":
		return token{kind: tokNull, text: text, value: nil, pos: start}, nil
	default:
		return token{kind: tokIdent, text: text, pos: start}, nil
	}
}

func (l *lexer) lexString() (token, error) {
	start := l.pos
	quote := l.input[l.pos]
	l.pos++
	var sb strings.Builder
	for l.pos < len(l.input) {
		c := l.input[l.pos]
		if c == '\\' {
			l.pos++
			if l.pos >= len(l.input) {
				return token{}, fmt.Errorf("%w: unterminated escape at %d", ErrSyntax, start)
			}
			esc := l.input[l.pos]
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '\\', '"', '\'':
				sb.WriteByte(esc)
			default:
				sb.WriteByte(esc)
			}
			l.pos++
			continue
		}
		if c == quote {
			l.pos++
			return token{kind: tokString, text: sb.String(), value: sb.String(), pos: start}, nil
		}
		sb.WriteByte(c)
		l.pos++
	}
	return token{}, fmt.Errorf("%w: unterminated string at %d", ErrSyntax, start)
}

func (l *lexer) lexNumber() (token, error) {
	start := l.pos
	if l.input[l.pos] == '-' {
		l.pos++
	}
	for l.pos < len(l.input) {
		c := l.input[l.pos]
		if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			l.pos++
			continue
		}
		break
	}
	raw := l.input[start:l.pos]
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return token{}, fmt.Errorf("%w: invalid number %q at %d", ErrSyntax, raw, start)
	}
	return token{kind: tokNumber, text: raw, value: v, pos: start}, nil
}

func (l *lexer) lexOperator() (token, error) {
	start := l.pos
	c := l.input[l.pos]
	l.pos++
	op := string(c)
	if l.pos < len(l.input) && l.input[l.pos] == '=' {
		op += "="
		l.pos++
	}
	switch op {
	case "==", "!=", ">", ">=", "<", "<=":
		return token{kind: tokOp, text: op, pos: start}, nil
	case "=":
		return token{kind: tokOp, text: "==", pos: start}, nil
	case "!":
		return token{}, fmt.Errorf("%w: unexpected %q at %d", ErrSyntax, op, start)
	default:
		return token{}, fmt.Errorf("%w: invalid operator %q at %d", ErrSyntax, op, start)
	}
}
