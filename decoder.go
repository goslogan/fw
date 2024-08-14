package fw

import (
	"bufio"
	"bytes"
	"encoding"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	columnTagName = "column"
	format        = "format"
)

type valueSetter func(field reflect.Value, structField reflect.StructField, rawValue string) error

// FWColumn defines the name and start and end points for a column in the data.
// This can be used to initialise a decoder when the columns are not defined in
// the first line of input text.
type FWColumn struct {
	Name        string
	Start       int
	End         int
	setter      valueSetter
	structField *reflect.StructField
	fieldNum    int
}

// A Decoder reads and decodes fixed width data from an input stream.
type Decoder struct {
	scanner            *bufio.Scanner
	lineTerminator     []byte
	fieldSeparator     string
	done               bool
	headersParsed      bool
	skipHeaders        bool
	settersInitialised bool
	cols               []*FWColumn
	headersLength      int
	structType         reflect.Type
	isPointer          bool
	lineNum            int
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	dec := &Decoder{
		scanner:        bufio.NewScanner(r),
		lineTerminator: []byte("\n"),
		fieldSeparator: " ",
	}
	dec.scanner.Split(dec.scan)
	return dec
}

// Unmarshal decodes a buffer into the array pointed to by v
func Unmarshal(buf []byte, v interface{}) error {
	return NewDecoder(bytes.NewReader(buf)).Decode(v)
}

// UnmarshalReader decodes a reader into the array pointed to by v
func UnmarshalReader(r io.Reader, v interface{}) error {
	return NewDecoder(r).Decode(v)
}

func (d *Decoder) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, d.lineTerminator); i >= 0 {
		// We have a full newline-terminated line.
		return i + len(d.lineTerminator), data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

// So we can check if a type implements TextUnmarsheler
var textUnmarshalerType = reflect.TypeOf(new(encoding.TextUnmarshaler)).Elem()

// Decode reads from its input and stores the decoded data to the value
// pointed to by v.
//
// v must point to a slice of structs (or pointers to structs)
//
// Currently, the maximum decodable line length is bufio.MaxScanTokenSize-1. ErrTooLong
// is returned if a line is encountered that too long to decode.
func (d *Decoder) Decode(v interface{}) error {

	var (
		err error
		ok  bool
	)

	if d.done {
		return fmt.Errorf("processing already complete")
	}

	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return ErrIncorrectInputValue
	}

	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Slice {

		if err := d.initialiseDecoder(rv); err != nil {
			return err
		}

		err, ok = d.readLines(rv)

	} else {

		if rv.Kind() != reflect.Struct {
			return ErrIncorrectInputValue
		}

		if err := d.initialiseDecoder(rv); err != nil {
			return err
		}

		err, ok = d.readLine(rv)

	}

	if d.done && err == nil && !ok {
		// d.done means we've reached the end of the file. err == nil && !ok
		// indicates that there was no data to read, so we propagate an io.EOF
		// upwards so our caller knows there is no data left.
		return io.EOF
	}

	return err
}

// Initialise setters and headers if required
func (d *Decoder) initialiseDecoder(v reflect.Value) error {

	d.structType = v.Type()

	if v.Kind() == reflect.Slice {
		d.structType = v.Type().Elem()
		if d.structType.Kind() == reflect.Pointer {
			d.isPointer = true
			d.structType = d.structType.Elem()
		}
		if d.structType.Kind() != reflect.Struct {
			return ErrIncorrectInputValue
		}

		if d.structType.Kind() != reflect.Struct {
			return ErrIncorrectInputValue
		}
	}

	if !d.headersParsed {
		colNames := d.getColNames()
		if err := d.parseHeaders(colNames); err != nil {
			return err
		}
		d.headersParsed = true
	}

	if !d.settersInitialised {
		if err := d.constructSetters(); err != nil {
			return err
		}
		d.settersInitialised = true
	}

	return nil
}

// At this point we *know* that v is a pointer to a slice.
func (d *Decoder) readLines(slice reflect.Value) (error, bool) {

	for {
		nv := reflect.New(d.structType).Elem()
		err, ok := d.readLine(nv)
		if err != nil {
			return err, false
		}
		if ok {
			if d.isPointer {
				slice.Set(reflect.Append(slice, nv.Addr()))
			} else {
				slice.Set(reflect.Append(slice, nv))
			}
		}
		if d.done {
			break
		}
	}
	return nil, true

}
func (d *Decoder) readLine(item reflect.Value) (error, bool) {

	ok := d.scanner.Scan()
	if !ok {
		if d.scanner.Err() != nil {
			return d.scanner.Err(), false
		}

		d.done = true
		return nil, false
	}

	d.lineNum++

	line := []rune(d.scanner.Text())
	if len(line) != d.headersLength {
		return fmt.Errorf("wrong data length in line %d", d.lineNum), false
	}

	for _, col := range d.cols {
		if col != nil {
			rawValue := string(line[col.Start:col.End])
			rawValue = strings.TrimSpace(rawValue)
			fieldVal := item.Field(col.fieldNum)
			if err := col.setter(fieldVal, *col.structField, rawValue); err != nil {
				return err, false
			}
		}
	}

	return nil, true

}

func (d *Decoder) parseHeaders(columnNames []string) error {

	if d.headersParsed {
		return nil
	}

	ok := d.scanner.Scan()
	if !ok {
		if d.scanner.Err() != nil {
			return d.scanner.Err()
		}

		d.done = true
		return nil
	}
	d.lineNum++

	if d.skipHeaders {
		return nil
	}

	line := d.scanner.Text()
	d.headersLength = len([]rune(line))

	d.cols = make([]*FWColumn, 0, len(columnNames))
	for i := 0; i < len(columnNames); i++ {
		colName := columnNames[i]
		re, err := regexp.Compile(fmt.Sprintf("(%s(?:%s+|$))", colName, d.fieldSeparator))
		if err != nil {
			return fmt.Errorf("%s column parsing error: %w", colName, err)
		}

		loc := re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		col := FWColumn{
			Name:  colName,
			Start: loc[0],
			End:   loc[1],
		}
		d.cols = append(d.cols, &col)
	}
	return nil
}

func (d *Decoder) constructSetters() error {

	nFields := d.structType.NumField()
	for fieldIndex := 0; fieldIndex < nFields; fieldIndex++ {
		currentField := d.structType.Field(fieldIndex)
		if currentField.IsExported() {
			tagName := getRefName(currentField)
			colDef := d.getColDef(tagName)
			if colDef != nil {
				colDef.fieldNum = fieldIndex
				err := fieldSetter(colDef, currentField)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil

}

// we know that v is a pointer to a slice of structs
// or pointers to structs.
func (d *Decoder) getColNames() []string {

	nFields := d.structType.NumField()
	colNames := make([]string, nFields)

	for n := 0; n < nFields; n++ {
		sf := d.structType.Field(n)
		colNames[n] = getRefName(sf)
	}

	return colNames

}

func getRefName(field reflect.StructField) string {
	if name, ok := field.Tag.Lookup(columnTagName); ok {
		return name
	}

	return field.Name
}

func (d *Decoder) getColDef(fieldName string) *FWColumn {
	for _, def := range d.cols {
		if def.Name == fieldName {
			return def
		}
	}

	return nil
}

// SetLineTerminator sets the character(s) that will be used to terminate lines.
//
// The default value is "\n".
func (d *Decoder) SetLineTerminator(lineTerminator []byte) {
	if len(lineTerminator) > 0 {
		d.lineTerminator = lineTerminator
	}
}

// SetFieldSeparator sets the character(s) that will be used to separate columns.
//
// The default value is a space.
func (d *Decoder) SetFieldSeparator(fieldSeparator string) {
	if fieldSeparator != "" {
		d.fieldSeparator = fieldSeparator
	}
}

// SetSkipHeaders can be used to set whether or not the first line is ignored
// By default, it is not skipped. If SetColumns is called, headers will be skipped.
// It may then be desirable to reset it. If SetColumns has been called, the headers
// will be read and discarded if SetSkipHeaders(true) is called.
func (d *Decoder) SetSkipHeaders(skip bool) {
	d.skipHeaders = skip
}

// SetColumns sets up the column names and widths to be used when parsing lines
// rather than extracting them from the first line of text.

func (d *Decoder) SetColumns(cols []*FWColumn) {
	if len(cols) > 0 {
		d.headersParsed = true
		d.skipHeaders = true
		d.settersInitialised = false
		d.cols = cols
	}
}

func fieldSetter(col *FWColumn, field reflect.StructField) error {

	col.structField = &field

	fieldKind := field.Type.Kind()
	isPointer := fieldKind == reflect.Ptr
	if isPointer {
		fieldKind = field.Type.Elem().Kind()
	}

	switch fieldKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if isPointer {
			col.setter = intSetPointer
		} else {
			col.setter = intSet
		}
	case reflect.Float32, reflect.Float64:
		if isPointer {
			col.setter = floatSetPointer
		} else {
			col.setter = floatSet
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if isPointer {
			col.setter = uintSetPointer
		} else {
			col.setter = uintSet
		}
	case reflect.String:
		if isPointer {
			col.setter = stringSetPointer
		} else {
			col.setter = stringSet
		}
	case reflect.Bool:
		if isPointer {
			col.setter = boolSetPointer
		} else {
			col.setter = boolSet
		}
	case reflect.Struct:
		if field.Type == reflect.TypeOf(time.Time{}) || field.Type == reflect.TypeOf(&time.Time{}) {
			if isPointer {
				col.setter = timeSetPointer
			} else {
				col.setter = timeSet
			}
			return nil
		}
		fallthrough
	default:
		if field.Type.Implements(textUnmarshalerType) {
			col.setter = textUnmarshalerSet
		} else if reflect.PointerTo(field.Type).Implements(textUnmarshalerType) {
			col.setter = textUnmarshalerSetPointer
		} else {
			return &InvalidUnmarshalError{Type: field.Type}
		}
	}

	return nil
}

func timeSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	timeFormat, ok := structField.Tag.Lookup(format)
	if !ok {
		timeFormat = time.RFC3339
	}
	t, err := time.Parse(timeFormat, rawValue)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}
	field.Set(reflect.ValueOf(t))
	return nil
}

func timeSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	timeFormat, ok := structField.Tag.Lookup(format)
	if !ok {
		timeFormat = time.RFC3339
	}
	t, err := time.Parse(timeFormat, rawValue)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}
	field.Set(reflect.ValueOf(&t))
	return nil
}

func uintSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	rawValue = strings.TrimSpace(rawValue)
	value, err := strconv.ParseUint(rawValue, 10, 64)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}
	v := reflect.New(field.Type().Elem())
	if v.Elem().OverflowUint(value) {
		return newOverflowError(value, structField)
	}
	v.Elem().SetUint(value)
	field.Set(v)
	return nil
}

func uintSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	rawValue = strings.TrimSpace(rawValue)
	value, err := strconv.ParseUint(rawValue, 10, 64)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}

	if field.OverflowUint(value) {
		return newOverflowError(value, structField)
	}
	field.SetUint(value)
	return nil
}

func intSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	value, err := strconv.ParseInt(rawValue, 10, 0)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}
	v := reflect.New(field.Type().Elem())
	if v.Elem().OverflowInt(value) {
		return newOverflowError(value, structField)
	}
	v.Elem().SetInt(value)
	field.Set(v)

	return nil
}

func intSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	value, err := strconv.ParseInt(rawValue, 10, 0)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}

	if field.OverflowInt(value) {
		return newOverflowError(value, structField)
	}
	field.SetInt(value)

	return nil
}

func floatSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}
	v := reflect.New(field.Type().Elem())
	if v.Elem().OverflowFloat(value) {
		return newOverflowError(value, structField)
	}
	v.Elem().SetFloat(value)
	field.Set(v)

	return nil
}

func floatSet(field reflect.Value, structField reflect.StructField, rawValue string) error {
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}

	if field.OverflowFloat(value) {
		return newOverflowError(value, structField)
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

	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return newCastingError(err, rawValue, structField)
	}
	field.SetBool(value)
	return nil
}

func boolSetPointer(field reflect.Value, structField reflect.StructField, rawValue string) error {

	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return newCastingError(err, rawValue, structField)
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

func newCastingError(err error, rawValue string, structField reflect.StructField) error {
	return fmt.Errorf(`failed casting "%s" to "%s:%v": %w`, rawValue, structField.Name, structField.Type, err)
}

func newOverflowError(value any, structField reflect.StructField) error {
	return fmt.Errorf(`value %v is too big for field %s:%v`, value, structField.Name, structField.Type)
}
