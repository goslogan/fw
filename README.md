# Fixed width file parser (decoder) for GO (golang)
[![License](http://img.shields.io/:license-mit-blue.svg)](LICENSE)


Updated library derived from [Oleg Lobanov's fwencoder](https://github.com/o1egl/fwencoder) with some aspects of [Ian Lopshire's go-fixedwidth](github.com/ianlopshire/go-fixedwidth)

Ths version currently only decodes but has a few additional features.

1. It supports the TextMarshaler/TextUnmarshaler interface
2. It allows multiple calls to the decoder by allowing a pointer to a struct to be passed to it as well as a slice.
3. It's slightly faster because it caches conversion functions
4. It supports arbitrary record endings and field conversions 
5. It allows the headers to be predefined by the caller 

* It **does not** support JSON decoding for complex data structures.
* **Encoding** is also unsupported. 

This library is using to parse fixed-width table data like:

```
Name            Address               Postcode Phone          Credit Limit Birthday
Evan Whitehouse V4560 Camel Back Road 3122     (918) 605-5383    1000000.5 19870101
Chuck Norris    P.O. Box 872          77868    (713) 868-6003     10909300 19651203
```

## Install

To install the library use the following command:

```
$ go get -u github.com/goslogan/fw
```

## Simple example

Parsing data from io.Reader:

```go
type Person struct {
	Name        string
	Address     string
	Postcode    int
	Phone       string
	CreditLimit float64   `json:"Credit Limit"`
	Bday        time.Time `column:"Birthday" format:"20060102"`
}

input, _ := os.ReadFile("/path/to/file")
defer f.Close

var people []Person
err := fw.Unmarshal(input, &people)
```

