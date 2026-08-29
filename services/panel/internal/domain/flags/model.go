package flags

import (
	"database/sql/driver"
	"time"

	"github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/users"
	"github.com/goccy/go-json"
)

type (
	TypeFlag  string
	ValueFlag []byte
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
	if len(v) == 0 {
		return false
	}

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

func (v ValueFlag) MarshalJSON() ([]byte, error) {
	return v, nil
}

func (v *ValueFlag) UnmarshalJSON(data []byte) error {
	out := make([]byte, len(data))
	copy(out, data)
	*v = out
	return nil
}

func (v *ValueFlag) Scan(src any) error {
	switch s := src.(type) {
	case []byte:
		return v.UnmarshalJSON(s)
	case string:
		*v = []byte(s)
		return nil
	}
	return ErrInvalidValue
}

func (v ValueFlag) Value() (driver.Value, error) {
	return string(v), nil
}

type Flag struct {
	ID           int64      `db:"id"`
	Key          string     `db:"key"`
	Name         string     `db:"name"`
	Type         TypeFlag   `db:"type"`
	DefaultValue ValueFlag  `db:"default_value"`
	Description  *string    `db:"description"`
	CreatedBy    int64      `db:"created_by"`
	UpdatedBy    int64      `db:"updated_by"`
	DeletedAt    *time.Time `db:"deleted_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

type FlagWithCreatorAndUpdater struct {
	Flag      Flag
	CreatedBy *users.User
	UpdatedBy *users.User
}
