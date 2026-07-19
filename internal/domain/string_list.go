package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringList stores ordered multi-value metadata as portable JSON text.
// Using text instead of a database-specific array keeps SQLite and PostgreSQL
// migrations and API semantics identical.
type StringList []string

func (values StringList) Value() (driver.Value, error) {
	if values == nil {
		values = StringList{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode string list: %w", err)
	}
	return string(encoded), nil
}

func (values *StringList) Scan(value any) error {
	if value == nil {
		*values = StringList{}
		return nil
	}

	var encoded []byte
	switch typed := value.(type) {
	case []byte:
		encoded = typed
	case string:
		encoded = []byte(typed)
	default:
		return fmt.Errorf("scan string list from %T", value)
	}
	if len(encoded) == 0 {
		*values = StringList{}
		return nil
	}
	if err := json.Unmarshal(encoded, values); err != nil {
		return fmt.Errorf("decode string list: %w", err)
	}
	if *values == nil {
		*values = StringList{}
	}
	return nil
}

func (StringList) GormDataType() string {
	return "text"
}

func (values StringList) MarshalJSON() ([]byte, error) {
	if values == nil {
		return []byte("[]"), nil
	}
	type plainStringList StringList
	return json.Marshal(plainStringList(values))
}
