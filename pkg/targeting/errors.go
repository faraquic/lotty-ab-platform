package targeting

import "errors"

// ErrSyntax is returned when a targeting expression cannot be parsed.
var ErrSyntax = errors.New("targeting syntax error")

// ErrInvalidAST is returned when serialized targeting AST is malformed.
var ErrInvalidAST = errors.New("invalid targeting AST")

// ErrEval is returned when targeting evaluation fails.
var ErrEval = errors.New("targeting evaluation error")
