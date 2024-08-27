package fw

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	String    string
	Bool      bool
	Int       int
	Int8      int8
	Int16     int16
	Int32     int32
	Int64     int64
	Uint      uint
	Uint8     uint8
	Uint16    uint16
	Uint32    uint32
	Uint64    uint64
	Float32   float32
	Float64   float64
	Date      time.Time `column:"Time"`
	Birthday  time.Time `column:"CustomDate" format:"02/01/2006"`
	PString   *string
	PBool     *bool
	PInt8     *int8
	PUint8    *uint8
	PFloat32  *float32
	PBirthday *time.Time `format:"02/01/2006"`
	Default   int
}

//go:embed "testdata/correct_all_supported.txt"
var byteData []byte

//go:embed "testdata/multi-line.txt"
var multiData []byte

//go:embed "testdata/multi-line-headless.txt"
var multiDataHeadless []byte

//go:embed "testdata/different-record-end.txt"
var differentRecord []byte

type DataSize struct {
	Value float64
	Units string
}

type DataValP struct {
	Name string
	Size *DataSize
}

type DataVal struct {
	Name string
	Size DataSize
}

func (datasize *DataSize) UnmarshalText(text []byte) error {
	var err error
	re := regexp.MustCompile(`(?i)(\d+(?:\.\d+))\s*(mi?b|gi?b|ti?b|ki?b|pi?b|zi?b)`)
	result := re.FindStringSubmatch(string(text))
	if len(result) == 3 {
		datasize.Units = result[2]
		datasize.Value, err = strconv.ParseFloat(result[1], 64)
	} else {
		return fmt.Errorf("fw: can't parse %s as datasize", string(text))
	}

	return err
}

func ExpectedTestStruct() TestStruct {
	strVal := "Test Ptr String"
	boolVal := false
	intVal := int8(15)
	uintVal := uint8(16)
	floatVal := float32(15.5)
	dateVal := time.Date(2017, 12, 28, 0, 0, 0, 0, time.UTC)

	return TestStruct{
		String:    "Test String",
		Bool:      true,
		Int:       -1,
		Int8:      -2,
		Int16:     -3,
		Int32:     -4,
		Int64:     -5,
		Uint:      1,
		Uint8:     2,
		Uint16:    3,
		Uint32:    4,
		Uint64:    5,
		Float32:   1.5,
		Float64:   2.5,
		Date:      time.Date(2017, 12, 27, 13, 48, 3, 0, time.UTC),
		Birthday:  time.Date(2017, 12, 27, 0, 0, 0, 0, time.UTC),
		PString:   &strVal,
		PBool:     &boolVal,
		PInt8:     &intVal,
		PUint8:    &uintVal,
		PFloat32:  &floatVal,
		PBirthday: &dateVal,
	}
}

func TestDecodeToStruct(t *testing.T) {
	obtained := TestStruct{}
	expected := ExpectedTestStruct()
	err := Unmarshal(byteData, &obtained)
	assert.Nil(t, err, "error unmarshalling: %v", err)
	assert.Equal(t, expected, obtained)
}

func TestDecodeToSliceOfStructs(t *testing.T) {
	obtained := []TestStruct{}
	expected := ExpectedTestStruct()

	err := Unmarshal(byteData, &obtained)

	assert.Nil(t, err, "error unmarshalling: %v", err)
	assert.Equal(t, []TestStruct{expected}, obtained)
}

func TestDecodeToSliceOfPointers(t *testing.T) {
	obtained := []*TestStruct{}
	expected := ExpectedTestStruct()

	err := Unmarshal(byteData, &obtained)

	assert.Nil(t, err, "error unmarshalling: %v", err)
	assert.Equal(t, []*TestStruct{&expected}, obtained)
}

func TestBadConversions(t *testing.T) {
	type BadData struct {
		Data  []byte
		Error string
	}

	badData := []BadData{
		{
			Data:  []byte("Int \n5.3 "),
			Error: `failed casting "5.3" to "Int:int"`,
		},
		{
			Data:  []byte("Uint8\n5123 "),
			Error: `is too big for field Uint8:uint8`,
		},

		{
			Data:  []byte("Bool\n5.3 "),
			Error: `failed casting "5.3" to "Bool:bool"`,
		},
		{
			Data:  []byte("Uint\n5.3 "),
			Error: `failed casting "5.3" to "Uint:uint"`,
		},
		{
			Data:  []byte("Float32\nhello  "),
			Error: `failed casting "hello" to "Float32:float32"`,
		},
		{
			Data:  []byte("Time\n5.3 "),
			Error: `failed casting "5.3" to "Date:time.Time"`,
		},
		{
			Data:  []byte("Int\n5"),
			Error: `wrong data length in line 2`,
		},
		{
			Data:  []byte("Int8\n5123"),
			Error: `is too big for field Int8:int8`,
		},

		{
			Data:  []byte(fmt.Sprintf("%-309s\n%.0f", "Float32", math.MaxFloat64)),
			Error: `is too big for field Float32:float32`,
		},
	}

	for _, data := range badData {
		var obtained []TestStruct
		t.Run(data.Error, func(t *testing.T) {
			err := Unmarshal(data.Data, &obtained)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), data.Error)
		})
	}
}

func TestBadInputs(t *testing.T) {
	type B struct {
		Int int `column:"Float32"`
	}

	type A struct {
		Float32 B
	}

	err := Unmarshal(nil, 1)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), ErrIncorrectInputValue.Error())

	err = Unmarshal(nil, nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), ErrIncorrectInputValue.Error())

	err = Unmarshal(nil, new(string))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), ErrIncorrectInputValue.Error())

	err = Unmarshal(nil, &([]int{}))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), ErrIncorrectInputValue.Error())

	err = Unmarshal([]byte("Float32\nhello  "), &([]A{}))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), `unable to create a converter for field "Float32" for type "fw.B"`)

}

func TestTextMarshal(t *testing.T) {

	data := "Name            Size          \ntest            20.5mb        "

	t.Run("pointer", func(t *testing.T) {
		expected := []DataValP{{
			Name: "test",
			Size: &DataSize{Value: 20.5, Units: "mb"},
		}}
		obtained := []DataValP{}
		err := Unmarshal([]byte(data), &obtained)
		assert.Nil(t, err)
		assert.Equal(t, expected, obtained)

	})

	t.Run("value", func(t *testing.T) {
		expected := []*DataVal{{
			Name: "test",
			Size: DataSize{Value: 20.5, Units: "mb"},
		}}

		obtained := []*DataVal{}
		err := Unmarshal([]byte(data), &obtained)
		assert.Nil(t, err)
		assert.Equal(t, expected, obtained)
	})

}

func TestMultipleStructs(t *testing.T) {

	type A struct {
		Alpha  string
		Number float32
		When   time.Time `column:"Date" format:"2006-01-02"`
	}

	type B struct {
		Beta   string
		Number float32
		When   time.Time `column:"Date" format:"2006-01-01"`
	}

	a := A{}
	b := B{}

	expectedA := A{Alpha: "𝜶", Number: 0.9, When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	expectedB := B{Beta: "β", Number: -1.4, When: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)}

	decoder := NewDecoder(bytes.NewReader(multiData))

	err := decoder.Decode(&a)
	assert.Nil(t, err)
	assert.Equal(t, expectedA, a)

	err = decoder.Decode(&b)
	assert.Nil(t, err)
	assert.Equal(t, expectedB, b)

}

func TestExplictHeaders(t *testing.T) {

	type A struct {
		Alpha  string
		Number float32
		When   time.Time `column:"Date" format:"2006-01-02"`
	}

	headers := map[string][]int{
		"Alpha":  {0, 7},
		"Beta":   {7, 13},
		"Number": {13, 26},
		"Date":   {26, 36},
	}
	expectedA := A{Alpha: "𝜶", Number: 0.9, When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}

	t.Run("skip", func(t *testing.T) {
		a := A{}

		decoder := NewDecoder(bytes.NewReader(multiData))
		decoder.SetHeaders(headers)
		decoder.SkipFirstRecord = true // we need to ignore the headers line

		err := decoder.Decode(&a)
		assert.Nil(t, err)
		assert.Equal(t, expectedA, a)
	})

	t.Run("noskip", func(t *testing.T) {
		a := A{}

		decoder := NewDecoder(bytes.NewReader(multiDataHeadless))
		decoder.SetHeaders(headers)

		err := decoder.Decode(&a)
		assert.Nil(t, err)
		assert.Equal(t, expectedA, a)
	})

}

func TestEndOfRecordMarker(t *testing.T) {

	type C struct {
		Alpha  string
		Beta   string
		Number float32
		When   time.Time `column:"Date" format:"2006-01-02"`
	}

	expected := []C{
		{Alpha: "𝜶", Beta: "Β", Number: 0.9, When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Alpha: "Α", Beta: "β", Number: -1.4, When: time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC)},
	}
	obtained := []C{}

	decoder := NewDecoder(bytes.NewReader(differentRecord))
	decoder.RecordTerminator = []byte{'|'}
	err := decoder.Decode(&obtained)

	assert.Nil(t, err)
	assert.Len(t, obtained, 2)
	assert.Equal(t, expected, obtained)
}
