package fw_test

import (
	_ "embed"
	"fmt"
	"math"
	"time"

	"github.com/goslogan/fw"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//go:embed "testdata/correct_all_supported.txt"
var byteData []byte

var _ = Describe("Decoder", Label("decode"), func() {

	It("unmarshal a single row of data into a struct", Label("struct"), func() {
		s := "Test Ptr String"
		bb := false
		i := int8(15)
		ui := uint8(16)
		f := float32(15.5)
		d := time.Date(2017, 12, 28, 0, 0, 0, 0, time.UTC)

		expected := []TestStruct{{
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
			PString:   &s,
			PBool:     &bb,
			PInt8:     &i,
			PUint8:    &ui,
			PFloat32:  &f,
			PBirthday: &d,
		}}

		var obtained []TestStruct
		Expect(fw.Unmarshal(byteData, &obtained)).NotTo(HaveOccurred())
		Expect(expected).To(Equal(obtained))
	})

	It("can decode a single row of data into a pointer to a struct", Label("decode", "pointer"), func() {

		s := "Test Ptr String"
		bb := false
		i := int8(15)
		ui := uint8(16)
		f := float32(15.5)
		d := time.Date(2017, 12, 28, 0, 0, 0, 0, time.UTC)

		expected := []*TestStruct{{
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
			PString:   &s,
			PBool:     &bb,
			PInt8:     &i,
			PUint8:    &ui,
			PFloat32:  &f,
			PBirthday: &d,
		}}

		var obtained []*TestStruct
		Expect(fw.Unmarshal(byteData, &obtained)).NotTo(HaveOccurred())
		Expect(expected).To(Equal(obtained))

	})
})

var _ = Describe("decode fail", Label("decode", "failure", "conversion"), func() {

	It("will fail on bad type conversions", func() {
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
			err := fw.Unmarshal(data.Data, &obtained)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(data.Error))
		}

	})

})

var _ = Describe("failure on input errors", Label("decode", "failure", "input"), func() {

	It("fails on bad inputs", func() {
		errs := []error{
			fw.Unmarshal(nil, 1),
			fw.Unmarshal(nil, nil),
			fw.Unmarshal(nil, new(string)),
			fw.Unmarshal(nil, &([]int{})),
		}

		for _, err := range errs {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fw.ErrIncorrectInputValue.Error()))
		}

		type B struct {
			Int int `column:")Float32"`
		}

		type A struct {
			Float32 B
		}

		err := fw.Unmarshal([]byte("Float32\nhello  "), &([]A{}))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("fw: Unmarshal(non-pointer fw_test.B)"))

		err = fw.Unmarshal([]byte("Float32\nhello  "), &([]B{}))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("error parsing regexp"))

	})

	var _ = Describe("TextUnmarshal", Label("decoder", "textmarshal"), func() {

		It("can decode a struct with a pointer member implementing TextUnmarshal", func() {

			expected := []DataValP{{
				Name: "test",
				Size: &DataSize{Value: 20.5, Units: "mb"},
			}}

			data := "Name            Size          \ntest            20.5mb        "

			actual := []DataValP{}
			Expect(fw.Unmarshal([]byte(data), &actual)).NotTo(HaveOccurred())
			Expect(expected).To(Equal(actual))
		})

		It("can decode a struct with a value member implementing TextUnmarshal", func() {

			expected := []DataVal{{
				Name: "test",
				Size: DataSize{Value: 20.5, Units: "mb"},
			}}

			data := "Name            Size          \ntest            20.5mb        "

			actual := []DataVal{}
			Expect(fw.Unmarshal([]byte(data), &actual)).NotTo(HaveOccurred())
			Expect(expected).To(Equal(actual))
		})

		It("can decode a pointer to astruct with a pointer member implementing TextUnmarshal", func() {

			expected := []*DataValP{{
				Name: "test",
				Size: &DataSize{Value: 20.5, Units: "mb"},
			}}

			data := "Name            Size          \ntest            20.5mb        "

			actual := []*DataValP{}
			Expect(fw.Unmarshal([]byte(data), &actual)).NotTo(HaveOccurred())
			Expect(expected).To(Equal(actual))
		})

		It("can decode a struct with a value member implementing TextUnmarshal", func() {

			expected := []*DataVal{{
				Name: "test",
				Size: DataSize{Value: 20.5, Units: "mb"},
			}}

			data := "Name            Size          \ntest            20.5mb        "

			actual := []*DataVal{}
			Expect(fw.Unmarshal([]byte(data), &actual)).NotTo(HaveOccurred())
			Expect(expected).To(Equal(actual))
		})

	})

})
