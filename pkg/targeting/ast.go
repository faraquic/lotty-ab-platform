// Package targeting implements the targeting expression DSL used by
// experiments. The DSL is parsed into a canonical JSON AST which is the
// representation persisted in the database.
//
// Grammar:
//
//	expr       := or_expr
//	or_expr    := and_expr ("OR" and_expr)*
//	and_expr   := unary_expr ("AND" unary_expr)*
//	unary_expr := "NOT" unary_expr | primary
//	primary    := comparison | "(" expr ")"
//	comparison := field operator literal | field "IN" array | field "NOT IN" array
//	operator   := "==" | "!=" | ">" | ">=" | "<" | "<="
package targeting

import "github.com/goccy/go-json"

// NodeType identifies the shape of an AST node.
type NodeType string

const (
	NodeOr  NodeType = "or"
	NodeAnd NodeType = "and"
	NodeNot NodeType = "not"
	NodeCmp NodeType = "cmp"
	NodeIn  NodeType = "in"
)

// Node is a targeting expression tree. The zero value is not valid; use
// Parse or Canonicalize to build one.
type Node struct {
	Type     NodeType `json:"type"`
	Children []Node   `json:"children,omitempty"`
	Child    *Node    `json:"child,omitempty"`
	Field    string   `json:"field,omitempty"`
	Operator string   `json:"operator,omitempty"`
	Value    any      `json:"value,omitempty"`
	Values   []any    `json:"values,omitempty"`
	Negated  bool     `json:"negated,omitempty"`
}

func marshalNode(n Node) ([]byte, error) {
	return json.Marshal(n)
}

func unmarshalNode(raw []byte) (Node, error) {
	var n Node
	if err := json.Unmarshal(raw, &n); err != nil {
		return Node{}, err
	}
	return n, nil
}
