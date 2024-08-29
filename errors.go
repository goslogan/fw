package fw

import (
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

// An InvalidLengthError describes the state of decoding when a data record
// does not have the same length as the headers indicated.
type InvalidLengthError struct {
	Headers       map[string][]int
	Line          string
	LineNum       int
	HeadersLength int
}

func (err *InvalidLengthError) Error() string {
	return fmt.Sprintf("wrong data length in line %d (%d != %d)",
		err.LineNum, len(err.Line), err.HeadersLength)

}

// An InvalidInputError is returned when the input to Decode is not
// usable
type InvalidInputError struct {
	Type reflect.Type
}

func (err *InvalidInputError) Error() string {
	t := "<nil>"
	if err.Type != nil {
		t = err.Type.String()
	}
	return fmt.Sprintf("input value is not a non-nil pointer to slice of structs or a pointer to a struct: %s", t)
}

type InvalidTypeError struct {
	Field reflect.StructField
}

func (err *InvalidTypeError) Error() string {
	return fmt.Sprintf(`unable to create a converter for field "%s" for type "%v"`, err.Field.Name, err.Field.Type)
}

type CastingError struct {
	Value string
	Err   error
	Field reflect.StructField
}

func (err *CastingError) Error() string {
	return fmt.Sprintf(`failed casting "%s" to "%s:%v": %+v`, err.Value, err.Field.Name, err.Field.Type, err.Err)
}

type OverflowError struct {
	Value interface{}
	Field reflect.StructField
}

func (err *OverflowError) Error() string {
	return fmt.Sprintf(`value %v is too big for field %s:%v`, err.Value, err.Field.Name, err.Field.Type)
}
