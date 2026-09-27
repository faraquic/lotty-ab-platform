package targeting

import (
	"testing"
)

func TestEvaluateEmpty(t *testing.T) {
	for _, raw := range [][]byte{nil, {}, []byte("  "), []byte("null"), []byte("{}"), []byte("  {}  ")} {
		got, err := Evaluate(raw, nil)
		if err != nil {
			t.Fatalf("Evaluate(%q): %v", raw, err)
		}
		if !got {
			t.Errorf("Evaluate(%q): expected true", raw)
		}
	}
}

func TestEvaluateStringComparison(t *testing.T) {
	ast := []byte(`{"type":"cmp","field":"country","operator":"==","value":"DE"}`)
	cases := []struct {
		attrs map[string]any
		want  bool
	}{
		{map[string]any{"country": "DE"}, true},
		{map[string]any{"country": "US"}, false},
		{map[string]any{}, false},
		{map[string]any{"country": nil}, false},
	}
	for _, tc := range cases {
		got, err := Evaluate(ast, tc.attrs)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("Evaluate(%v): got %v, want %v", tc.attrs, got, tc.want)
		}
	}
}

func TestEvaluateNumberComparison(t *testing.T) {
	ast := []byte(`{"type":"cmp","field":"age","operator":">=","value":18}`)
	cases := []struct {
		attrs map[string]any
		want  bool
	}{
		{map[string]any{"age": 18}, true},
		{map[string]any{"age": 25}, true},
		{map[string]any{"age": 17}, false},
		{map[string]any{"age": 18.5}, true},
		{map[string]any{}, false},
	}
	for _, tc := range cases {
		got, err := Evaluate(ast, tc.attrs)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("Evaluate(%v): got %v, want %v", tc.attrs, got, tc.want)
		}
	}
}

func TestEvaluateInNotIn(t *testing.T) {
	inAst := []byte(`{"type":"in","field":"plan","values":["free","pro"],"negated":false}`)
	notInAst := []byte(`{"type":"in","field":"plan","values":["free"],"negated":true}`)

	cases := []struct {
		ast   []byte
		attrs map[string]any
		want  bool
	}{
		{inAst, map[string]any{"plan": "free"}, true},
		{inAst, map[string]any{"plan": "pro"}, true},
		{inAst, map[string]any{"plan": "enterprise"}, false},
		{inAst, map[string]any{}, false},
		{notInAst, map[string]any{"plan": "free"}, false},
		{notInAst, map[string]any{"plan": "pro"}, true},
		{notInAst, map[string]any{}, true},
	}
	for _, tc := range cases {
		got, err := Evaluate(tc.ast, tc.attrs)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("Evaluate(%v): got %v, want %v", tc.attrs, got, tc.want)
		}
	}
}

func TestEvaluateLogicalOperators(t *testing.T) {
	andAst := []byte(`{"type":"and","children":[{"type":"cmp","field":"country","operator":"==","value":"DE"},{"type":"cmp","field":"age","operator":">=","value":18}]}`)
	orAst := []byte(`{"type":"or","children":[{"type":"cmp","field":"country","operator":"==","value":"DE"},{"type":"cmp","field":"country","operator":"==","value":"US"}]}`)
	notAst := []byte(`{"type":"not","child":{"type":"cmp","field":"country","operator":"==","value":"DE"}}`)

	cases := []struct {
		ast   []byte
		attrs map[string]any
		want  bool
	}{
		{andAst, map[string]any{"country": "DE", "age": 25}, true},
		{andAst, map[string]any{"country": "DE", "age": 17}, false},
		{andAst, map[string]any{"country": "US", "age": 25}, false},
		{orAst, map[string]any{"country": "DE"}, true},
		{orAst, map[string]any{"country": "US"}, true},
		{orAst, map[string]any{"country": "FR"}, false},
		{notAst, map[string]any{"country": "DE"}, false},
		{notAst, map[string]any{"country": "US"}, true},
		{notAst, map[string]any{}, true},
	}
	for _, tc := range cases {
		got, err := Evaluate(tc.ast, tc.attrs)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("Evaluate(%v): got %v, want %v", tc.attrs, got, tc.want)
		}
	}
}

func TestEvaluateNullComparison(t *testing.T) {
	eqNull := []byte(`{"type":"cmp","field":"email","operator":"==","value":null}`)
	neNull := []byte(`{"type":"cmp","field":"email","operator":"!=","value":null}`)

	cases := []struct {
		ast   []byte
		attrs map[string]any
		want  bool
	}{
		{eqNull, map[string]any{}, true},
		{eqNull, map[string]any{"email": nil}, true},
		{eqNull, map[string]any{"email": "test@example.com"}, false},
		{neNull, map[string]any{}, false},
		{neNull, map[string]any{"email": nil}, false},
		{neNull, map[string]any{"email": "test@example.com"}, true},
	}
	for _, tc := range cases {
		got, err := Evaluate(tc.ast, tc.attrs)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("Evaluate(%v): got %v, want %v", tc.attrs, got, tc.want)
		}
	}
}

func TestEvaluateTypeCoercion(t *testing.T) {
	ast := []byte(`{"type":"cmp","field":"age","operator":">=","value":18}`)
	cases := []struct {
		attrs map[string]any
		want  bool
	}{
		{map[string]any{"age": "25"}, true},
		{map[string]any{"age": "17"}, false},
		{map[string]any{"age": "abc"}, false},
	}
	for _, tc := range cases {
		got, err := Evaluate(ast, tc.attrs)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("Evaluate(%v): got %v, want %v", tc.attrs, got, tc.want)
		}
	}
}

func TestEvaluateBoolComparison(t *testing.T) {
	ast := []byte(`{"type":"cmp","field":"active","operator":"==","value":true}`)
	cases := []struct {
		attrs map[string]any
		want  bool
	}{
		{map[string]any{"active": true}, true},
		{map[string]any{"active": false}, false},
		{map[string]any{}, false},
	}
	for _, tc := range cases {
		got, err := Evaluate(ast, tc.attrs)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("Evaluate(%v): got %v, want %v", tc.attrs, got, tc.want)
		}
	}
}

func TestEvaluateNestedExpression(t *testing.T) {
	ast := []byte(`{"type":"or","children":[{"type":"and","children":[{"type":"cmp","field":"country","operator":"==","value":"DE"},{"type":"cmp","field":"age","operator":">=","value":18}]},{"type":"cmp","field":"plan","operator":"==","value":"enterprise"}]}`)
	cases := []struct {
		attrs map[string]any
		want  bool
	}{
		{map[string]any{"country": "DE", "age": 25}, true},
		{map[string]any{"country": "DE", "age": 17}, false},
		{map[string]any{"plan": "enterprise"}, true},
		{map[string]any{"plan": "free"}, false},
	}
	for _, tc := range cases {
		got, err := Evaluate(ast, tc.attrs)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("Evaluate(%v): got %v, want %v", tc.attrs, got, tc.want)
		}
	}
}

func TestEvaluateInvalidAST(t *testing.T) {
	_, err := Evaluate([]byte(`{"type":"unknown"}`), nil)
	if err == nil {
		t.Error("expected error for unknown node type")
	}
}
