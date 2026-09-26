package targeting

import (
	"strings"
	"testing"
)

func TestParseAndFormatRoundTrip(t *testing.T) {
	cases := []string{
		`country == "DE"`,
		`country != "US" AND plan IN ["free", "pro"]`,
		`NOT (age >= 18)`,
		`a == 1 OR b == 2 AND NOT c == "x"`,
		`plan NOT IN ["free"]`,
		`score > 10 AND score <= 20`,
	}
	for _, in := range cases {
		node, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		raw, err := marshalNode(node)
		if err != nil {
			t.Fatalf("marshal(%q): %v", in, err)
		}
		formatted, err := Format(raw)
		if err != nil {
			t.Fatalf("Format(%q): %v", in, err)
		}
		if _, err := Parse(formatted); err != nil {
			t.Fatalf("re-parse(%q -> %q): %v", in, formatted, err)
		}
	}
}

func TestParsePrecedence(t *testing.T) {
	node, err := Parse(`a == 1 OR b == 2 AND c == 3`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeOr {
		t.Fatalf("expected OR at root, got %s", node.Type)
	}
	if node.Children[1].Type != NodeAnd {
		t.Fatalf("expected AND as right child, got %s", node.Children[1].Type)
	}
}

func TestFormatAddsParens(t *testing.T) {
	node, err := Parse(`(a == 1 OR b == 2) AND c == 3`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	raw, _ := marshalNode(node)
	out, err := Format(raw)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(out, "(a == 1 OR b == 2)") {
		t.Fatalf("expected parenthesized or-group, got %q", out)
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		"",
		"   ",
		"a ==",
		"a",
		"== 1",
		"(a == 1",
		"a IN",
		"a IN [",
		"a IN [1,",
		"a ~ 1",
		`a == "unterminated`,
		"a == 1 AND",
	}
	for _, in := range bad {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q): expected error, got nil", in)
		}
	}
}

func TestInNotIn(t *testing.T) {
	node, err := Parse(`plan NOT IN ["free", "trial"]`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeIn || !node.Negated {
		t.Fatalf("expected negated IN, got %+v", node)
	}
	if len(node.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(node.Values))
	}
}

func TestCanonicalJSON(t *testing.T) {
	raw, err := CanonicalJSON([]byte(`country == "DE"`))
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	if !strings.HasPrefix(string(raw), "{") {
		t.Fatalf("expected JSON object, got %s", raw)
	}

	if _, err := CanonicalJSON([]byte("")); err != nil {
		t.Fatalf("empty should be fine: %v", err)
	}
	if _, err := CanonicalJSON([]byte("{}")); err != nil {
		t.Fatalf("empty object should be fine: %v", err)
	}

	existing := []byte(`{"type":"cmp","field":"country","operator":"==","value":"DE"}`)
	out, err := CanonicalJSON(existing)
	if err != nil {
		t.Fatalf("existing AST: %v", err)
	}
	if string(out) != string(existing) {
		t.Fatalf("existing AST should pass through, got %s", out)
	}
}

func TestFormatLiteralNumbers(t *testing.T) {
	out, err := Format([]byte(`age >= 18 AND ratio < 0.5`))
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(out, "18") || !strings.Contains(out, "0.5") {
		t.Fatalf("numbers not preserved: %q", out)
	}
}
