package targeting

import (
	"fmt"
	"math"
	"strconv"
)

// Evaluate evaluates a canonical targeting AST against subject attributes.
// Returns true if the subject matches the targeting criteria.
// Missing attributes cause non-match (fail closed).
func Evaluate(astJSON []byte, attrs map[string]any) (bool, error) {
	if IsEmpty(astJSON) {
		return true, nil
	}
	node, err := unmarshalNode(astJSON)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrEval, err)
	}
	return evalNode(node, attrs)
}

func evalNode(n Node, attrs map[string]any) (bool, error) {
	switch n.Type {
	case NodeOr:
		if len(n.Children) != 2 {
			return false, fmt.Errorf("%w: or node must have two children", ErrEval)
		}
		left, err := evalNode(n.Children[0], attrs)
		if err != nil {
			return false, err
		}
		right, err := evalNode(n.Children[1], attrs)
		if err != nil {
			return false, err
		}
		return left || right, nil
	case NodeAnd:
		if len(n.Children) != 2 {
			return false, fmt.Errorf("%w: and node must have two children", ErrEval)
		}
		left, err := evalNode(n.Children[0], attrs)
		if err != nil {
			return false, err
		}
		right, err := evalNode(n.Children[1], attrs)
		if err != nil {
			return false, err
		}
		return left && right, nil
	case NodeNot:
		if n.Child == nil {
			return false, fmt.Errorf("%w: not node missing child", ErrEval)
		}
		v, err := evalNode(*n.Child, attrs)
		if err != nil {
			return false, err
		}
		return !v, nil
	case NodeCmp:
		return evalCmp(n, attrs)
	case NodeIn:
		return evalIn(n, attrs)
	default:
		return false, fmt.Errorf("%w: unknown node type %q", ErrEval, n.Type)
	}
}

func evalCmp(n Node, attrs map[string]any) (bool, error) {
	attrVal, exists := attrs[n.Field]

	if n.Value == nil {
		switch n.Operator {
		case "==":
			return !exists || attrVal == nil, nil
		case "!=":
			return exists && attrVal != nil, nil
		default:
			return false, fmt.Errorf("%w: operator %q not supported for null", ErrEval, n.Operator)
		}
	}

	if !exists || attrVal == nil {
		return false, nil
	}

	return compareValues(attrVal, n.Operator, n.Value)
}

func evalIn(n Node, attrs map[string]any) (bool, error) {
	attrVal, exists := attrs[n.Field]

	if !exists || attrVal == nil {
		return n.Negated, nil
	}

	found := false
	for _, v := range n.Values {
		if v == nil {
			continue
		}
		ok, err := compareValues(attrVal, "==", v)
		if err != nil {
			return false, err
		}
		if ok {
			found = true
			break
		}
	}

	if n.Negated {
		return !found, nil
	}
	return found, nil
}

func compareValues(attrVal any, operator string, astVal any) (bool, error) {
	if isNumber(attrVal) && isNumber(astVal) {
		return compareNumbers(toFloat(attrVal), operator, toFloat(astVal))
	}
	if isString(attrVal) && isString(astVal) {
		return compareStrings(toString(attrVal), operator, toString(astVal))
	}
	if isBool(attrVal) && isBool(astVal) {
		return compareBools(toBool(attrVal), operator, toBool(astVal))
	}
	if isString(attrVal) && isNumber(astVal) {
		f, err := strconv.ParseFloat(toString(attrVal), 64)
		if err != nil {
			return false, nil
		}
		return compareNumbers(f, operator, toFloat(astVal))
	}
	if isNumber(attrVal) && isString(astVal) {
		f, err := strconv.ParseFloat(toString(astVal), 64)
		if err != nil {
			return false, nil
		}
		return compareNumbers(toFloat(attrVal), operator, f)
	}
	return false, nil
}

func compareNumbers(a float64, op string, b float64) (bool, error) {
	switch op {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	default:
		return false, fmt.Errorf("%w: unknown operator %q", ErrEval, op)
	}
}

func compareStrings(a, op, b string) (bool, error) {
	switch op {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	default:
		return false, fmt.Errorf("%w: unknown operator %q", ErrEval, op)
	}
}

func compareBools(a bool, op string, b bool) (bool, error) {
	switch op {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	default:
		return false, fmt.Errorf("%w: operator %q not supported for bool", ErrEval, op)
	}
}

func isNumber(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int8:
		return float64(n)
	case int16:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint:
		return float64(n)
	case uint8:
		return float64(n)
	case uint16:
		return float64(n)
	case uint32:
		return float64(n)
	case uint64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	default:
		return math.NaN()
	}
}

func isString(v any) bool {
	_, ok := v.(string)
	return ok
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func isBool(v any) bool {
	_, ok := v.(bool)
	return ok
}

func toBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
