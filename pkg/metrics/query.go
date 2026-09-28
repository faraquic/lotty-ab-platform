package metrics

import (
	"fmt"
	"strconv"
	"strings"
)

type QueryBuilder struct{}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{}
}

func (b *QueryBuilder) Build(node Node) (string, error) {
	return b.build(node)
}

func (b *QueryBuilder) build(n Node) (string, error) {
	switch n.Type {
	case NodeNumber:
		return strconv.FormatFloat(n.Value, 'f', -1, 64), nil

	case NodeField:
		if !allowedFields[n.Field] {
			return "", fmt.Errorf("unknown field %q", n.Field)
		}
		return n.Field, nil

	case NodeBinary:
		if len(n.Children) != 2 {
			return "", fmt.Errorf("binary op requires 2 children")
		}
		left, err := b.build(n.Children[0])
		if err != nil {
			return "", err
		}
		right, err := b.build(n.Children[1])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", left, n.Op, right), nil

	case NodeFunc:
		return b.buildFunc(n)

	default:
		return "", fmt.Errorf("unknown node type %q", n.Type)
	}
}

func (b *QueryBuilder) buildFunc(n Node) (string, error) {
	switch n.Name {
	case "count":
		return "count()", nil

	case "sum":
		if len(n.Children) != 1 {
			return "", fmt.Errorf("sum expects 1 arg")
		}
		child, err := b.build(n.Children[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("sum(%s)", child), nil

	case "unique_count":
		if len(n.Children) != 1 {
			return "", fmt.Errorf("unique_count expects 1 arg")
		}
		child, err := b.build(n.Children[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("uniqExact(%s)", child), nil

	case "avg":
		if len(n.Children) != 1 {
			return "", fmt.Errorf("avg expects 1 arg")
		}
		child, err := b.build(n.Children[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("avg(%s)", child), nil

	case "percentile", "quantile":
		if len(n.Children) != 2 {
			return "", fmt.Errorf("%s expects 2 args", n.Name)
		}
		level, err := b.build(n.Children[0])
		if err != nil {
			return "", err
		}
		field, err := b.build(n.Children[1])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("quantileTDigest(%s, %s)", level, field), nil

	default:
		return "", fmt.Errorf("unknown function %q", n.Name)
	}
}

func (b *QueryBuilder) BuildMetric(metricType string, field string, level float64) (string, error) {
	switch metricType {
	case "count":
		return "count()", nil
	case "sum":
		return fmt.Sprintf("sum(%s)", field), nil
	case "unique_count":
		return fmt.Sprintf("uniqExact(%s)", field), nil
	case "average":
		return fmt.Sprintf("avg(%s)", field), nil
	case "percentile":
		return fmt.Sprintf("quantileTDigest(%s, %s)", strconv.FormatFloat(level, 'f', -1, 64), field), nil
	case "ratio":
		return "", fmt.Errorf("ratio requires numerator and denominator")
	default:
		return "", fmt.Errorf("unknown metric type %q", metricType)
	}
}

func (b *QueryBuilder) BuildRatio(numerator, denominator string) string {
	return fmt.Sprintf("(%s) / nullif((%s), 0)", numerator, denominator)
}

func SanitizeIdentifier(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
