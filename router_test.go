package clacks

import (
	"reflect"
	"testing"
)

type privateStuff struct{}

func (p *privateStuff) privateMethod(j privateStuff) privateStuff {
	return j
}

type MyService struct{}

func (m *MyService) Func1(a int, b string, j *MyService) error {
	return nil
}

type MyServiceError1 struct{}

func (m *MyServiceError1) Func1(a int, b string) int {
	return a
}

type MyServiceError2 struct{}

func (m *MyServiceError2) Func1(a privateStuff, b MyServiceError1) error {
	return nil
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

func checkExportedFails(st interface{}, t *testing.T) {
	objType := reflect.TypeOf(st)
	methodType := objType.Method(0).Type
	_, err := exportedMethods(methodType)
	if err != nil {
		t.Error("Allows a private object argument to be exported")
	}

}

func TestMethodArguments(t *testing.T) {
	objType := reflect.TypeOf(new(MyService))
	methodType := objType.Method(0).Type
	methodArgs, err := searchMethodArguments(methodType)
	if err != nil {
		t.Error(err)
	}
	expected := []methodArgument{methodArgument{"int", reflect.TypeOf(0), false},
		methodArgument{"string", reflect.TypeOf(""), false},
		methodArgument{"MyService", reflect.TypeOf(MyService{}), true}}
	if !reflect.DeepEqual(methodArgs, expected) {
		t.Error("Got method arguments different from expected")
	}
}

func TestExportedMethods(t *testing.T) {
	methods, err := exportedMethods(reflect.TypeOf(new(MyService)))
	if err != nil {
		t.Error("Valid service does have an error in its methods")
	}
	expected := make(map[string]*methodData)
	expected["Func1"] = &methodData{
		method: reflect.TypeOf(new(MyService)).Method(0),
		args: []methodArgument{methodArgument{"int", reflect.TypeOf(0), false},
			methodArgument{"string", reflect.TypeOf(""), false},
			methodArgument{"MyService", reflect.TypeOf(MyService{}), true}}}
	if !reflect.DeepEqual(methods, expected) {
		t.Error("Didn't get any method as exportable")
	}

	checkExportedFails(new(privateStuff), t)
	checkExportedFails(new(MyServiceError1), t)
	checkExportedFails(new(MyServiceError2), t)
}

func TestRegister(t *testing.T) {
	router := new(Router)
	mysp := new(MyService)

	err := router.Register(MyService{})
	if err == nil {
		t.Error("MyService without methods can be registerd (is not pointer)")
	}

	err = router.Register(mysp)
	if err != nil {
		t.Error("Could not register MyService")
	}
	expectedMethods := make(map[string]*methodData)
	expectedMethods["Func1"] = &methodData{
		method: reflect.TypeOf(mysp).Method(0),
		args: []methodArgument{methodArgument{"int", reflect.TypeOf(0), false},
			methodArgument{"string", reflect.TypeOf(""), false},
			methodArgument{"MyService", reflect.TypeOf(MyService{}), true}}}
	expected := make(map[string]*service)
	expected["MyService"] = &service{
		name:    "MyService",
		rcvr:    reflect.ValueOf(mysp),
		typ:     reflect.TypeOf(mysp),
		methods: expectedMethods}
	if !reflect.DeepEqual(router.svcMap, expected) {
		t.Error("After registration the map doesn't contain what is expected")
	}

	err = router.Register(privateStuff{})
	if err == nil {
		t.Error("privateStuff can be registered!")
	}
	err = router.Register(MyService{})
	if err == nil {
		t.Error("MyService without methods can be registered twice!")
	}
	err = router.Register(MyServiceError1{})
	if err == nil {
		t.Error("MyServiceError1 without methods can be registered")
	}
}
