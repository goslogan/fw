package fw

import (
	"errors"
	"reflect"
)

// An InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
// (The argument to Unmarshal must be a non-nil pointer.)
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "fw: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "fw: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "fw: Unmarshal(nil " + e.Type.String() + ")"
}

// ErrIncorrectInputValue represents wrong input param
var ErrIncorrectInputValue = errors.New("value is not a pointer to slice of structs")
