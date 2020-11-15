package test

import phone "github.com/idleroamer/goqface/tests/Phone/dependent/Tests/Phone"

//go:generate python3 ../../../codegen.py --src ../ --input dependent/Phone.qface
//go:generate gofmt -w Tests

type PhoneAdapter struct {
	*phone.Phone
}
