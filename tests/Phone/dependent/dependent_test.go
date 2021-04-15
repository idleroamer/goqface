package test

import phone "github.com/idleroamer/goqface/tests/Phone/dependent/Tests/Phone"

//go:generate python3 ../../../generator/codegen.py --dependency ../dependency --input Phone.qface
//go:generate gofmt -w Tests

type PhoneImpl struct {
	*phone.PhoneAdapter
}
