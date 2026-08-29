package flags

import (
	"time"

	"github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/users"
	"github.com/goccy/go-json"
)

type (
	TypeFlag  string
	ValueFlag json.RawMessage
)

const (
	TypeFlagString TypeFlag = "string"
	TypeFlagNumber TypeFlag = "number"
	TypeFlagBool   TypeFlag = "bool"
)

func (t TypeFlag) Valid() bool {
	switch t {
	case TypeFlagString, TypeFlagNumber, TypeFlagBool:
		return true
	default:
		return false
	}
}

func (v ValueFlag) Valid(flagType TypeFlag) bool {
	if !json.Valid(v) {
		return false
	}

	var value any
	if err := json.Unmarshal(v, &value); err != nil {
		return false
	}

	switch flagType {
	case TypeFlagString:
		_, ok := value.(string)
		return ok
	case TypeFlagNumber:
		_, ok := value.(float64)
		return ok
	case TypeFlagBool:
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
}

type Flag struct {
	ID           int64
	Key          string
	Type         TypeFlag
	DefaultValue ValueFlag
	Description  string
	Owner        int64
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type FlagWithOwner struct {
	Flag  Flag
	Owner *users.User
}
