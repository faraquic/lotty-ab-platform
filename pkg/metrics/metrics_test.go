package metrics

import (
	"testing"
)

func TestParse_Number(t *testing.T) {
	node, err := Parse("42")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeNumber || node.Value != 42 {
		t.Errorf("got %+v, want number 42", node)
	}
}

func TestParse_Field(t *testing.T) {
	node, err := Parse("latency")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeField || node.Field != "latency" {
		t.Errorf("got %+v, want field latency", node)
	}
}

func TestParse_BinaryAdd(t *testing.T) {
	node, err := Parse("a + b")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeBinary || node.Op != "+" {
		t.Errorf("got %+v, want binary +", node)
	}
	if len(node.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(node.Children))
	}
}

func TestParse_BinaryPrecedence(t *testing.T) {
	node, err := Parse("a + b * c")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeBinary || node.Op != "+" {
		t.Errorf("got %+v, want binary +", node)
	}
	if node.Children[1].Type != NodeBinary || node.Children[1].Op != "*" {
		t.Errorf("right child = %+v, want binary *", node.Children[1])
	}
}

func TestParse_Parentheses(t *testing.T) {
	node, err := Parse("(a + b) * c")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeBinary || node.Op != "*" {
		t.Errorf("got %+v, want binary *", node)
	}
	if node.Children[0].Type != NodeBinary || node.Children[0].Op != "+" {
		t.Errorf("left child = %+v, want binary +", node.Children[0])
	}
}

func TestParse_FuncCall(t *testing.T) {
	node, err := Parse("sum(event_value)")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeFunc || node.Name != "sum" {
		t.Errorf("got %+v, want func sum", node)
	}
	if len(node.Children) != 1 {
		t.Fatalf("got %d children, want 1", len(node.Children))
	}
}

func TestParse_Percentile(t *testing.T) {
	node, err := Parse("percentile(0.95, latency)")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node.Type != NodeFunc || node.Name != "percentile" {
		t.Errorf("got %+v, want func percentile", node)
	}
	if len(node.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(node.Children))
	}
	if node.Children[0].Value != 0.95 {
		t.Errorf("level = %f, want 0.95", node.Children[0].Value)
	}
}

func TestParse_EmptyFormula(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Fatal("Parse = nil, want error")
	}
}

func TestParse_InvalidSyntax(t *testing.T) {
	_, err := Parse("a +")
	if err == nil {
		t.Fatal("Parse = nil, want error")
	}
}

func TestValidate_ValidFormula(t *testing.T) {
	node, err := Parse("conversion_count / exposure_count")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(node); err != nil {
		t.Errorf("Validate = %v, want nil", err)
	}
}

func TestValidate_UnknownFunc(t *testing.T) {
	node, err := Parse("malicious()")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(node); err == nil {
		t.Fatal("Validate = nil, want error")
	}
}

func TestValidate_UnknownField(t *testing.T) {
	node, err := Parse("unknown_field")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(node); err == nil {
		t.Fatal("Validate = nil, want error")
	}
}

func TestValidate_WrongArgCount(t *testing.T) {
	node, err := Parse("sum(a, b)")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(node); err == nil {
		t.Fatal("Validate = nil, want error")
	}
}

func TestQueryBuilder_Count(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.Build(Node{Type: NodeFunc, Name: "count"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if expr != "count()" {
		t.Errorf("expr = %q, want count()", expr)
	}
}

func TestQueryBuilder_Sum(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.Build(Node{
		Type:     NodeFunc,
		Name:     "sum",
		Children: []Node{{Type: NodeField, Field: "event_value"}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if expr != "sum(event_value)" {
		t.Errorf("expr = %q, want sum(event_value)", expr)
	}
}

func TestQueryBuilder_UniqueCount(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.Build(Node{
		Type:     NodeFunc,
		Name:     "unique_count",
		Children: []Node{{Type: NodeField, Field: "subject_id"}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if expr != "uniqExact(subject_id)" {
		t.Errorf("expr = %q, want uniqExact(subject_id)", expr)
	}
}

func TestQueryBuilder_Avg(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.Build(Node{
		Type:     NodeFunc,
		Name:     "avg",
		Children: []Node{{Type: NodeField, Field: "latency"}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if expr != "avg(latency)" {
		t.Errorf("expr = %q, want avg(latency)", expr)
	}
}

func TestQueryBuilder_Percentile(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.Build(Node{
		Type: NodeFunc,
		Name: "percentile",
		Children: []Node{
			{Type: NodeNumber, Value: 0.95},
			{Type: NodeField, Field: "latency"},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if expr != "quantileTDigest(0.95, latency)" {
		t.Errorf("expr = %q, want quantileTDigest(0.95, latency)", expr)
	}
}

func TestQueryBuilder_BinaryOp(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.Build(Node{
		Type: NodeBinary,
		Op:   "/",
		Children: []Node{
			{Type: NodeFunc, Name: "count"},
			{Type: NodeFunc, Name: "count"},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if expr != "(count() / count())" {
		t.Errorf("expr = %q, want (count() / count())", expr)
	}
}

func TestQueryBuilder_BuildMetricCount(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.BuildMetric("count", "", 0)
	if err != nil {
		t.Fatalf("BuildMetric: %v", err)
	}
	if expr != "count()" {
		t.Errorf("expr = %q, want count()", expr)
	}
}

func TestQueryBuilder_BuildMetricSum(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.BuildMetric("sum", "event_value", 0)
	if err != nil {
		t.Fatalf("BuildMetric: %v", err)
	}
	if expr != "sum(event_value)" {
		t.Errorf("expr = %q, want sum(event_value)", expr)
	}
}

func TestQueryBuilder_BuildMetricPercentile(t *testing.T) {
	qb := NewQueryBuilder()
	expr, err := qb.BuildMetric("percentile", "latency", 0.95)
	if err != nil {
		t.Fatalf("BuildMetric: %v", err)
	}
	if expr != "quantileTDigest(0.95, latency)" {
		t.Errorf("expr = %q, want quantileTDigest(0.95, latency)", expr)
	}
}

func TestQueryBuilder_BuildRatio(t *testing.T) {
	qb := NewQueryBuilder()
	expr := qb.BuildRatio("countIf(event_type = 'conversion')", "countIf(event_type = 'exposure')")
	expected := "(countIf(event_type = 'conversion')) / nullif((countIf(event_type = 'exposure')), 0)"
	if expr != expected {
		t.Errorf("expr = %q, want %q", expr, expected)
	}
}

func TestExpandBuiltins_ConversionRate(t *testing.T) {
	node, err := Parse("conversion_rate")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	expanded := ExpandBuiltins(node)
	if expanded.Type != NodeBinary || expanded.Op != "/" {
		t.Errorf("got %+v, want binary /", expanded)
	}
}

func TestExpandBuiltins_NestedFormula(t *testing.T) {
	node, err := Parse("conversion_rate * 100")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	expanded := ExpandBuiltins(node)
	if expanded.Type != NodeBinary || expanded.Op != "*" {
		t.Errorf("got %+v, want binary *", expanded)
	}
	if expanded.Children[0].Type != NodeBinary || expanded.Children[0].Op != "/" {
		t.Errorf("left child = %+v, want binary /", expanded.Children[0])
	}
}

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"event_value", "event_value"},
		{"event-value", "eventvalue"},
		{"event value", "eventvalue"},
		{"event;value", "eventvalue"},
		{"", ""},
	}
	for _, tt := range tests {
		got := SanitizeIdentifier(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
