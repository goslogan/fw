package fw

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"regexp"
)

const (
	columnTagName = "column"
	format        = "format"
)

// A Decoder reads and decodes fixed width data from an input stream.
type Decoder struct {
	scanner        *bufio.Scanner
	lineTerminator []byte
	fieldSeparator string
	done           bool
	headersParsed  bool
	headersLength  int
	skipHeaders    bool
	lineNum        int
	headers        map[string][]int
	lastType       reflect.Type
	lastSetter     structSetter
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

		structType := rv.Type().Elem()
		if structType.Kind() == reflect.Pointer {
			structType = structType.Elem()
		}
		if structType.Kind() != reflect.Struct {
			return ErrIncorrectInputValue
		}

		if err := d.initialiseDecoder(); err != nil {
			return err
		}

		err, ok = d.readLines(rv)

	} else {

		if rv.Kind() != reflect.Struct {
			return ErrIncorrectInputValue
		}

		if err := d.initialiseDecoder(); err != nil {
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
func (d *Decoder) initialiseDecoder() error {

	if !d.headersParsed {
		if err := d.parseHeaders(); err != nil {
			return err
		}
		d.headersParsed = true
	}

	return nil
}

// At this point we *know* that v is a pointer to a slice.
func (d *Decoder) readLines(slice reflect.Value) (error, bool) {

	structType := slice.Type().Elem()
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}

	for {
		nv := reflect.New(structType).Elem()
		err, ok := d.readLine(nv)
		if err != nil {
			return err, false
		}
		if ok {
			if slice.Type().Elem().Kind() == reflect.Pointer {
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
	line := d.scanner.Text()
	lineLen := len([]rune(line))
	t := item.Type()

	if lineLen != d.headersLength {
		return fmt.Errorf("wrong data length in line %d (%d != %d)", d.lineNum, lineLen, d.headersLength), false
	}

	if t != d.lastType {
		var err error
		d.lastType = t
		d.lastSetter, err = cachedStructSetter(t, d.headers, d.fieldSeparator)
		if err != nil {
			return err, false
		}
	}

	return d.lastSetter(item, line), true

}

func (d *Decoder) parseHeaders() error {

	if d.headersParsed {
		return nil
	}

	headerRegexp, err := regexp.Compile(fmt.Sprintf(".+?(?:%s+|$)", d.fieldSeparator))
	if err != nil {
		return err
	}
	// this won't fail if above didn't
	trimRegexp, _ := regexp.Compile(fmt.Sprintf("%s+", d.fieldSeparator))

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

	indices := headerRegexp.FindAllStringIndex(line, -1)
	d.headers = make(map[string][]int)
	for _, index := range indices {
		header := line[index[0]:index[1]]
		d.headers[trimRegexp.ReplaceAllString(header, "")] = index
	}

	d.headersParsed = true
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
