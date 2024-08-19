package fw

import (
	"errors"
	"fmt"
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
var ErrIncorrectInputValue = errors.New("value is not a pointer to slice of structs or a pointer to a struct")

func newCastingError(err error, rawValue string, structField reflect.StructField) error {
	return fmt.Errorf(`failed casting "%s" to "%s:%v": %w`, rawValue, structField.Name, structField.Type, err)
}

func newOverflowError(value any, structField reflect.StructField) error {
	return fmt.Errorf(`value %v is too big for field %s:%v`, value, structField.Name, structField.Type)
}
