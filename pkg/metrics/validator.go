package metrics

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidFormula = errors.New("invalid formula")
	ErrUnknownFunc    = errors.New("unknown function")
	ErrUnknownField   = errors.New("unknown field")
	ErrInvalidArgs    = errors.New("invalid arguments")
)

var allowedFuncs = map[string]int{
	"count":            0,
	"sum":              1,
	"unique_count":     1,
	"avg":              1,
	"percentile":       2,
	"quantile":         2,
	"exposure_count":   0,
	"conversion_count": 0,
	"error_count":      0,
}

var allowedFields = map[string]bool{
	"event_value":      true,
	"latency":          true,
	"subject_id":       true,
	"duration":         true,
	"revenue":          true,
	"event_type":       true,
	"conversion_count": true,
	"exposure_count":   true,
	"error_count":      true,
	"conversion_rate":  true,
	"error_rate":       true,
	"avg_latency":      true,
	"p95_latency":      true,
	"p99_latency":      true,
}

func Validate(node Node) error {
	return validateNode(node)
}

func validateNode(n Node) error {
	switch n.Type {
	case NodeNumber:
		return nil

	case NodeField:
		if !allowedFields[n.Field] {
			return fmt.Errorf("%w: %q", ErrUnknownField, n.Field)
		}
		return nil

	case NodeBinary:
		if n.Op != "+" && n.Op != "-" && n.Op != "*" && n.Op != "/" {
			return fmt.Errorf("%w: invalid operator %q", ErrInvalidFormula, n.Op)
		}
		if len(n.Children) != 2 {
			return fmt.Errorf("%w: binary op requires 2 children", ErrInvalidFormula)
		}
		if err := validateNode(n.Children[0]); err != nil {
			return err
		}
		return validateNode(n.Children[1])

	case NodeFunc:
		arity, ok := allowedFuncs[n.Name]
		if !ok {
			return fmt.Errorf("%w: %q", ErrUnknownFunc, n.Name)
		}
		if len(n.Children) != arity {
			return fmt.Errorf("%w: %s expects %d args, got %d", ErrInvalidArgs, n.Name, arity, len(n.Children))
		}
		for _, child := range n.Children {
			if err := validateNode(child); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("%w: unknown node type %q", ErrInvalidFormula, n.Type)
	}
}
