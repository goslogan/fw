// Package fw defines a model for converting fixed width input data into Redis structs.
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
// The caller can either define field sizes directly via [Decoder.SetHeaders] or they can be read
// from the first line of input.
//
// # Annotations
//
// Structs are annotated with the name of the input field/column with the column annotation. Referencing a column
// which does not exist will cause the field to be silently ignored during processing. Given the range of date/time
// formats in data, [time.Time] fields are supported additionally by the format annotation which allows the template
// for [time.ParseDate] to be provided.
//
// # Usable target structures
//
// The data structure passed to [Decoder.Decode] or [Unmarshal] must be a pointer to an existing slice or a pointer to a struct.
// If a slice is provided, it must contain structs or pointers to structs. It can be empty. Data is appended to the slice.
//
// All basic go data types are supported automatically. As mentioned above [time.Time] is supported explicitly. Any other
// data type must support the [encoding.TextUnmarshaler] interface.  Any other data type will cause an error to be returned.
type Decoder struct {
	scanner          *bufio.Scanner
	RecordTerminator []byte // RecordTerminator identifies the sequence of bytes used to indicate end of record (default is "\n")
	FieldSeparator   string // FieldSeparator is used to identify the characters between fields and also to trim those characters. It's used as part of a regular expression (default is a space)
	done             bool
	headersParsed    bool
	headersLength    int
	SkipFirstRecord  bool // SkipFirstRecord defines whether the first line should be ignored.
	// By default, it is not skipped. If SetColumns is called, headers will be skipped.
	// It may then be desirable to reset it. If SetColumns has been called, the headers
	// will be read and discarded if SkipFirstRecord is true
	lineNum    int
	headers    map[string][]int
	lastType   reflect.Type
	lastSetter structSetter
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	dec := &Decoder{
		scanner:          bufio.NewScanner(r),
		RecordTerminator: []byte("\n"),
		FieldSeparator:   " ",
	}
	dec.scanner.Split(dec.scan)
	return dec
}

// Unmarshal decodes a buffer into the array or structed pointed to by v
// If v is not an array only the first record will be read
func Unmarshal(buf []byte, v interface{}) error {
	return UnmarshalReader(bytes.NewReader(buf), v)
}

// UnmarshalReader decodes an io.Reader into the array or structed pointed to by v
// If v is not an array only the first record will be read
func UnmarshalReader(r io.Reader, v interface{}) error {
	return NewDecoder(r).Decode(v)
}

// Decode reads from its input and stores the decoded data to the value
// pointed to by v. v may point to a struct or a slice of structs (or pointers to structs)
//
// Currently, the maximum decodable line length is bufio.MaxScanTokenSize-1. ErrTooLong
// is returned if a line is encountered that too long to decode.
func (decoder *Decoder) Decode(v interface{}) error {

	var (
		err error
		ok  bool
	)

	if decoder.done {
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

		if err := decoder.parseHeaders(); err != nil {
			return err
		}

		err, ok = decoder.readLines(rv)

	} else {

		if rv.Kind() != reflect.Struct {
			return ErrIncorrectInputValue
		}

		if err := decoder.parseHeaders(); err != nil {
			return err
		}

		err, ok = decoder.readLine(rv)

	}

	if decoder.done && err == nil && !ok {
		// decoder.done means we've reached the end of the file. err == nil && !ok
		// indicates that there was no data to read, so we propagate an io.EOF
		// upwards so our caller knows there is no data left.
		return io.EOF
	}

	return err
}

// At this point we *know* that v is a pointer to a slice.
func (decoder *Decoder) readLines(slice reflect.Value) (error, bool) {

	structType := slice.Type().Elem()
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}

	for {
		nv := reflect.New(structType).Elem()
		err, ok := decoder.readLine(nv)
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
		if decoder.done {
			break
		}
	}
	return nil, true

}
func (decoder *Decoder) readLine(item reflect.Value) (error, bool) {

	ok := decoder.scanner.Scan()
	if !ok {
		if decoder.scanner.Err() != nil {
			return decoder.scanner.Err(), false
		}

		decoder.done = true
		return nil, false
	}

	decoder.lineNum++
	line := decoder.scanner.Text()
	lineLen := len([]rune(line))
	t := item.Type()

	if lineLen != decoder.headersLength {
		return fmt.Errorf("wrong data length in line %d (%d != %d)", decoder.lineNum, lineLen, decoder.headersLength), false
	}

	if t != decoder.lastType {
		var err error
		decoder.lastType = t
		decoder.lastSetter, err = cachedStructSetter(t, decoder.headers, decoder.FieldSeparator)
		if err != nil {
			return err, false
		}
	}

	return decoder.lastSetter(item, line), true

}

func (decoder *Decoder) parseHeaders() error {

	if decoder.headersParsed && !decoder.SkipFirstRecord {
		return nil
	}

	headerRegexp, err := regexp.Compile(fmt.Sprintf(".+?(?:%s+|$)", decoder.FieldSeparator))
	if err != nil {
		return err
	}
	// this won't fail if above didn't
	trimRegexp, _ := regexp.Compile(fmt.Sprintf("%s+", decoder.FieldSeparator))

	ok := decoder.scanner.Scan()
	if !ok {
		if decoder.scanner.Err() != nil {
			return decoder.scanner.Err()
		}

		decoder.done = true
		return nil
	}
	decoder.lineNum++

	// this may be called just to consume the header...
	if decoder.headersParsed && decoder.SkipFirstRecord {
		return nil
	}

	line := decoder.scanner.Text()
	decoder.headersLength = len([]rune(line))

	indices := headerRegexp.FindAllStringIndex(line, -1)
	decoder.headers = make(map[string][]int)
	for _, index := range indices {
		header := line[index[0]:index[1]]
		decoder.headers[trimRegexp.ReplaceAllString(header, "")] = index
	}

	decoder.headersParsed = true
	return nil
}

// SetHeaders overrides any headers parsed from the first line of input.
// If decoder.SetHeaders is called , decoder.SkipFirstRecord is set to false.
// If decoder.SkipFirstRecord is then set to true, the first line will be read
// but not parsed
func (decoder *Decoder) SetHeaders(headers map[string][]int) {
	decoder.headers = headers

	for _, v := range headers {
		if v[1] > decoder.headersLength {
			decoder.headersLength = v[1]
		}
	}

	decoder.headersParsed = true
	decoder.SkipFirstRecord = false
}

func (decoder *Decoder) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, decoder.RecordTerminator); i >= 0 {
		// We have a full newline-terminated line.
		return i + len(decoder.RecordTerminator), data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
