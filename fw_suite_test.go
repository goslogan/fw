package fw_test

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

func (d *DataSize) UnmarshalText(text []byte) error {
	var err error
	re := regexp.MustCompile(`(?i)(\d+(?:\.\d+))\s*(mi?b|gi?b|ti?b|ki?b|pi?b|zi?b)`)
	result := re.FindStringSubmatch(string(text))
	if len(result) == 3 {
		d.Units = result[2]
		d.Value, err = strconv.ParseFloat(result[1], 64)
	} else {
		return fmt.Errorf("fw: can't parse %s as datasize", string(text))
	}

	return err
}

func TestFw(t *testing.T) {
	suiteConfig, reportConfig := GinkgoConfiguration()
	RegisterFailHandler(Fail)
	suiteConfig.LabelFilter = "textmarshal"
	RunSpecs(t, "Fw Suit", suiteConfig, reportConfig)
}
