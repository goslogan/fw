package fw

import (
	"bytes"
	"fmt"
	"time"
)

func ExampleUnmarshal() {

	source := []byte("name    dob       \nPeter   2008-10-11\nNicki   1987-01-28")

	type Person struct {
		Name string    `column:"name"`
		DOB  time.Time `column:"dob" format:"2006-01-02"`
	}

	people := []Person{}

	err := Unmarshal(source, &people)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Number of records: %d\n", len(people))
	for _, person := range people {
		fmt.Printf("%s=%s\n", person.Name, person.DOB.Format(time.RFC822))
	}

	//Output: Number of records: 2
	//Peter=11 Oct 08 00:00 UTC
	//Nicki=28 Jan 87 00:00 UTC
}

func ExampleDecoder() {

	source := []byte("name    dob       \nPeter   2008-10-11\nNicki   1987-01-28")

	type Person struct {
		Name string    `column:"name"`
		DOB  time.Time `column:"dob" format:"2006-01-02"`
	}

	person := Person{}
	decoder := NewDecoder(bytes.NewBuffer(source))
	err := decoder.Decode(&person)

	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", person)
	//Output: {Name:Peter DOB:2008-10-11 00:00:00 +0000 UTC}
}

func ExampleDecoder_explicit() {
	source := []byte("Peter   2008-10-11\nNicki   1987-01-28")

	type Person struct {
		Name string    `column:"name"`
		DOB  time.Time `column:"dob" format:"2006-01-02"`
	}

	columns := map[string][]int{"name": {0, 8}, "dob": {8, 18}}

	person := Person{}
	decoder := NewDecoder(bytes.NewBuffer(source))
	decoder.SetHeaders(columns)
	err := decoder.Decode(&person)

	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", person)
	//Output: {Name:Peter DOB:2008-10-11 00:00:00 +0000 UTC}
}
