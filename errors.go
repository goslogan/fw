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

func (err *InvalidUnmarshalError) Error() string {
	if err.Type == nil {
		return "fw: Unmarshal(nil)"
	}

	if err.Type.Kind() != reflect.Ptr {
		return "fw: Unmarshal(non-pointer " + err.Type.String() + ")"
	}
	return "fw: Unmarshal(nil " + err.Type.String() + ")"
}

// ErrIncorrectInputValue represents wrong input param
var ErrIncorrectInputValue = errors.New("value is not a pointer to slice of structs or a pointer to a struct")

// newInvalidTypeError represents a type we convert.
func newInvalidTypeError(structField reflect.StructField) error {
	return fmt.Errorf(`unable to create a converter for field "%s" for type "%v"`, structField.Name, structField.Type)
}

func newCastingError(err error, rawValue string, structField reflect.StructField) error {
	return fmt.Errorf(`failed casting "%s" to "%s:%v": %w`, rawValue, structField.Name, structField.Type, err)
}

func newOverflowError(value any, structField reflect.StructField) error {
	return fmt.Errorf(`value %v is too big for field %s:%v`, value, structField.Name, structField.Type)
}
