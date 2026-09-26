package targeting

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Format renders persisted targeting bytes (DSL, canonical AST JSON or empty)
// as a human-readable DSL string. Empty targeting returns "".
func Format(raw []byte) (string, error) {
	if IsEmpty(raw) {
		return "", nil
	}
	trimmed := trimSpaceBytes(raw)
	var node Node
	if trimmed[0] == '{' {
		n, err := unmarshalNode(trimmed)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidAST, err)
		}
		node = n
	} else {
		n, err := Parse(string(trimmed))
		if err != nil {
			return "", err
		}
		node = n
	}
	return formatNode(node)
}

func formatNode(n Node) (string, error) {
	switch n.Type {
	case NodeOr:
		if len(n.Children) != 2 {
			return "", fmt.Errorf("%w: or node must have two children", ErrInvalidAST)
		}
		left, err := formatChild(n.Children[0], n.Type)
		if err != nil {
			return "", err
		}
		right, err := formatChild(n.Children[1], n.Type)
		if err != nil {
			return "", err
		}
		return left + " OR " + right, nil
	case NodeAnd:
		if len(n.Children) != 2 {
			return "", fmt.Errorf("%w: and node must have two children", ErrInvalidAST)
		}
		left, err := formatChild(n.Children[0], n.Type)
		if err != nil {
			return "", err
		}
		right, err := formatChild(n.Children[1], n.Type)
		if err != nil {
			return "", err
		}
		return left + " AND " + right, nil
	case NodeNot:
		if n.Child == nil {
			return "", fmt.Errorf("%w: not node missing child", ErrInvalidAST)
		}
		child, err := formatChild(*n.Child, NodeNot)
		if err != nil {
			return "", err
		}
		return "NOT " + child, nil
	case NodeCmp:
		return fmt.Sprintf("%s %s %s", n.Field, n.Operator, formatLiteral(n.Value)), nil
	case NodeIn:
		parts := make([]string, 0, len(n.Values))
		for _, v := range n.Values {
			parts = append(parts, formatLiteral(v))
		}
		op := "IN"
		if n.Negated {
			op = "NOT IN"
		}
		return fmt.Sprintf("%s %s [%s]", n.Field, op, strings.Join(parts, ", ")), nil
	default:
		return "", fmt.Errorf("%w: unknown node type %q", ErrInvalidAST, n.Type)
	}
}

// formatChild wraps child expressions in parentheses when they have lower or
// equal precedence than the parent to preserve semantics.
func formatChild(child Node, parent NodeType) (string, error) {
	s, err := formatNode(child)
	if err != nil {
		return "", err
	}
	if needsParens(child.Type, parent) {
		return "(" + s + ")", nil
	}
	return s, nil
}

func needsParens(child, parent NodeType) bool {
	rank := func(t NodeType) int {
		switch t {
		case NodeOr:
			return 0
		case NodeAnd:
			return 1
		case NodeNot:
			return 2
		default:
			return 3
		}
	}
	return rank(child) < rank(parent)
}

func formatLiteral(v any) string {
	switch value := v.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(value)
	case bool:
		if value {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			parts = append(parts, formatLiteral(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprint(value)
	}
}

func trimSpaceBytes(raw []byte) []byte {
	return bytes.TrimSpace(raw)
}
