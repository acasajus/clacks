package clacks

import (
	"reflect"
	"testing"
)

type privateStuff int

func privateMethod(p privateStuff) privateStuff {
	return p
}

type MyService int

func (m *MyService) Func1(a int, b string) (int, error) {
	return a, nil
}

type MyServiceError1 int

func (m *MyServiceError1) Func1(a int, b string) int {
	return a
}

/* TEST START */

func TestIsExported(t *testing.T) {
	if isExported("lol") {
		t.Error("lowercase is accepted as Exported")
	}
	if !isExported("Lol") {
		t.Error("Lowercase not is accepted as Exported")
	}
}

func TestIsExportedOrBuiltinType(t *testing.T) {
	if !isExportedOrBuiltinType(reflect.TypeOf(0)) {
		t.Error("Builtin is not found to be valid parameter")
	}
	typ := reflect.TypeOf(new(MyService))
	if !isExportedOrBuiltinType(typ) {
		t.Error("Uppercase struct is not found to be a valid parameter")
	}
	typ = reflect.TypeOf(new(privateStuff))
	if isExportedOrBuiltinType(typ) {
		t.Error("Lppercase struct is found to be a valid parameter")
	}
}

func TestMethodArguments(t *testing.T) {
	objType := reflect.TypeOf(new(MyService))
	methodType := objType.Method(0).Type
	methodArgs, err := methodArguments(methodType.In, methodType.NumIn())
	if err != nil {
		t.Error(err)
	}
	expected := []reflect.Type{reflect.TypeOf(0), reflect.TypeOf("")}
	if reflect.DeepEqual(methodArgs, expected) {
		t.Error("Got method arguments different from expected")
	}
	methodType = reflect.TypeOf(privateMethod)
	methodArgs, err = methodArguments(methodType.In, methodType.NumIn())
	if err == nil {
		t.Error("Allows a private object argument to be exported")
	}
}

func TestExportedMethods(t *testing.T) {
	methods, err := exportedMethods(reflect.TypeOf(new(MyService)))
	if err != nil {
		t.Error("Valid service does have an error in its methods")
	}
	expected := make(map[string]*methodData)
	expected["Func1"] = &methodData{
		method:      reflect.TypeOf(new(MyService)).Method(0),
		ArgsType:    []reflect.Type{reflect.TypeOf(0), reflect.TypeOf("")},
		RepliesType: []reflect.Type{reflect.TypeOf(0)}}
	if reflect.DeepEqual(methods, expected) {
		t.Error("Didn't get any method as exportable")
	}
	_, err = exportedMethods(reflect.TypeOf(new(MyServiceError1)))
	if err == nil {
		t.Error("Allowed struct with invalid methods for exporting")
	}
}
