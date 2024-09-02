package fw

import (
	"encoding"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type valueSetter func(field reflect.Value, structField reflect.StructField, rawValue string) error
type structSetter func(item reflect.Value, line string) error

// So we can check if a type implements TextUnmarsheler
var textUnmarshalerType = reflect.TypeOf(new(encoding.TextUnmarshaler)).Elem()

// getFieldSetter returns a setter if one can be found and nil if not
func getFieldSetter(field reflect.StructField) (valueSetter, error) {

	var setter valueSetter
	var err error

	fieldKind := field.Type.Kind()
	isPointer := fieldKind == reflect.Ptr
	if isPointer {
		fieldKind = field.Type.Elem().Kind()
	}

	// Special case for time.Time because it implements TextUnmarshaler but we need more
	// to handle the format annotation.
	if field.Type == reflect.TypeOf(time.Time{}) || field.Type == reflect.TypeOf(&time.Time{}) {
		if isPointer {
			return createTimeSetPointer(field), nil
		} else {
			return createTimeSet(field), nil
		}
	}

	if field.Type.Implements(textUnmarshalerType) {
		return textUnmarshalerSet, nil
	} else if reflect.PointerTo(field.Type).Implements(textUnmarshalerType) {
		return textUnmarshalerSetPointer, nil
	}

	switch fieldKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if isPointer {
			setter = intSetPointer
		} else {
			setter = intSet
		}
	case reflect.Float32, reflect.Float64:
		if isPointer {
			setter = floatSetPointer
		} else {
			setter = floatSet
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if isPointer {
			setter = uintSetPointer
		} else {
			setter = uintSet
		}
	case reflect.String:
		if isPointer {
			setter = stringSetPointer
		} else {
			setter = stringSet
		}
	case reflect.Bool:
		if isPointer {
			setter = boolSetPointer
		} else {
			setter = boolSet
		}
	default:
		err = &InvalidTypeError{Field: field}
	}

	return setter, err
}

func createTimeSet(structField reflect.StructField) valueSetter {

	timeFormat, ok := structField.Tag.Lookup(format)
	if !ok {
		timeFormat = time.RFC3339
	}

	return func(field reflect.Value, structField reflect.StructField, rawValue string) error {
		t, err := time.Parse(timeFormat, rawValue)
		if err != nil {
			return &CastingError{Err: err, Value: rawValue, Field: structField}
		}
		field.Set(reflect.ValueOf(t))
		return nil
	}
}

func createTimeSetPointer(structField reflect.StructField) valueSetter {

	timeFormat, ok := structField.Tag.Lookup(format)
	if !ok {
		timeFormat = time.RFC3339
	}
	return func(field reflect.Value, structField reflect.StructField, rawValue string) error {

		t, err := time.Parse(timeFormat, rawValue)
		if err != nil {
			return &CastingError{Err: err, Value: rawValue, Field: structField}
		}
		field.Set(reflect.ValueOf(&t))
		return nil
	}
}

func uintSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	rawValue = strings.TrimSpace(rawValue)
	value, err := strconv.ParseUint(rawValue, 10, 64)
	if err != nil {
		return &CastingError{Err: err, Value: rawValue, Field: structField}
	}
	v := reflect.New(field.Type().Elem())
	if v.Elem().OverflowUint(value) {
		return &OverflowError{Value: value, Field: structField}
	}
	v.Elem().SetUint(value)
	field.Set(v)
	return nil
}

func uintSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	rawValue = strings.TrimSpace(rawValue)
	value, err := strconv.ParseUint(rawValue, 10, 64)
	if err != nil {
		return &CastingError{Err: err, Value: rawValue, Field: structField}
	}

	if field.OverflowUint(value) {
		return &OverflowError{Value: value, Field: structField}
	}
	field.SetUint(value)
	return nil
}

func intSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	value, err := strconv.ParseInt(rawValue, 10, 0)
	if err != nil {
		return &CastingError{Err: err, Value: rawValue, Field: structField}
	}
	v := reflect.New(field.Type().Elem())
	if v.Elem().OverflowInt(value) {
		return &OverflowError{Value: value, Field: structField}
	}
	v.Elem().SetInt(value)
	field.Set(v)

	return nil
}

func intSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	value, err := strconv.ParseInt(rawValue, 10, 0)
	if err != nil {
		return &CastingError{Err: err, Value: rawValue, Field: structField}
	}

	if field.OverflowInt(value) {
		return &OverflowError{Value: value, Field: structField}
	}
	field.SetInt(value)

	return nil
}

func floatSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return &CastingError{Err: err, Value: rawValue, Field: structField}
	}
	v := reflect.New(field.Type().Elem())
	if v.Elem().OverflowFloat(value) {
		return &OverflowError{Value: value, Field: structField}
	}
	v.Elem().SetFloat(value)
	field.Set(v)

	return nil
}

func floatSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return &CastingError{Err: err, Value: rawValue, Field: structField}
	}

	if field.OverflowFloat(value) {
		return &OverflowError{Value: value, Field: structField}
	}
	field.SetFloat(value)

	return nil
}

func stringSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	field.SetString(rawValue)
	return nil
}

func stringSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	field.Set(reflect.ValueOf(&rawValue))
	return nil
}

func boolSet(field reflect.Value, structField reflect.StructField, rawValue string) error {

	value, err := parseBool(rawValue)

	if err != nil {
		return &CastingError{Err: err, Value: rawValue, Field: structField}
	}
	field.SetBool(value)
	return nil
}

func boolSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {

	value, err := parseBool(rawValue)
	if err != nil {
		return &CastingError{Err: err, Value: rawValue, Field: structField}
	}
	field.Set(reflect.ValueOf(&value))
	return nil
}

func textUnmarshalerSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	t := field.Type()
	if t.Kind() == reflect.Ptr && field.IsNil() {
		field.Set(reflect.New(t.Elem()))
	}
	return field.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(rawValue))
}

func textUnmarshalerSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	t := field.Type()
	field = field.Addr()
	// set to zero value if this is nil
	if t.Kind() == reflect.Ptr && field.IsNil() {
		field.Set(reflect.New(t.Elem()))
	}
	return field.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(rawValue))
}

func createStructSetter(st reflect.Type, indices map[string][]int, fieldSeparator string) (structSetter, error) {

	nFields := st.NumField()
	valueSetters := make([]func(reflect.Value, []rune) error, 0)
	leftTrimmer := regexp.MustCompile("^" + fieldSeparator + "+")
	rightTrimmer := regexp.MustCompile(fieldSeparator + "+$")

	for fieldIndex := 0; fieldIndex < nFields; fieldIndex++ {
		currentField := st.Field(fieldIndex)
		if currentField.IsExported() {
			tagName := getRefName(currentField)
			if index, ok := indices[tagName]; ok {
				setter, err := getFieldSetter(currentField)
				if err != nil {
					return nil, err
				}
				if setter != nil {
					valueSetters = append(valueSetters, valueSetterFunc(currentField, fieldIndex, index[0], index[1], leftTrimmer, rightTrimmer, setter))
				}
			}
		}
	}

	return structSetterFunc(valueSetters), nil

}

func structSetterFunc(valueSetters []func(reflect.Value, []rune) error) func(item reflect.Value, line string) error {
	return func(item reflect.Value, line string) error {
		lineRunes := []rune(line)
		for _, setter := range valueSetters {
			if err := setter(item, lineRunes); err != nil {
				return err
			}
		}
		return nil
	}
}

func valueSetterFunc(currentField reflect.StructField, idx, from, to int, leftTrimmer, rightTrimmer *regexp.Regexp, setter valueSetter) func(reflect.Value, []rune) error {
	return func(v reflect.Value, line []rune) error {
		fieldVal := v.Field(idx)
		fieldRunes := line[from:to]
		rawField := leftTrimmer.ReplaceAllString(string(fieldRunes), "")
		rawField = rightTrimmer.ReplaceAllString(rawField, "")
		return setter(fieldVal, currentField, rawField)
	}
}

func getRefName(field reflect.StructField) string {
	if name, ok := field.Tag.Lookup(columnTagName); ok {
		return name
	}

	return field.Name
}

func parseBool(str string) (bool, error) {
	switch str {
	case "yes", "YES", "Yes":
		return true, nil
	case "no", "NO", "No":
		return false, nil
	default:
		return strconv.ParseBool(str)
	}
}

var structSetterCache sync.Map // map[string]structSetter

func cachedStructSetter(t reflect.Type, indices map[string][]int, fieldSeparator string) (structSetter, error) {
	key := fmt.Sprintf("%s.%s:%v:%s", t.PkgPath(), t.Name(), indices, fieldSeparator)
	if f, ok := structSetterCache.Load(key); ok {
		return f.(structSetter), nil
	}
	setter, err := createStructSetter(t, indices, fieldSeparator)
	if err != nil {
		return nil, err
	}
	f, _ := structSetterCache.LoadOrStore(key, setter)
	return f.(structSetter), nil
}
