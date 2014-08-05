package clacks

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"code.google.com/p/go.net/context"
)

type privateStuff struct{}

func (p *privateStuff) privateMethod(j privateStuff) privateStuff {
	return j
}

type MyService struct{}
type TestData struct {
	A int
}

func (m *MyService) Func1(ctx context.Context, a int, b string, j *TestData) error {
	j.A = a
	return nil
}

type MyInvalidService1 struct{}

func (m *MyInvalidService1) Func1(a int, b string) int {
	return a
}

type MyInvalidService2 struct{}

func (m *MyInvalidService2) Func1() int {
	return 1
}

type MyInvalidService3 struct{}

func (m *MyInvalidService3) Func1(ctx context.Context, a int, b string) int {
	return a
}

type MyServiceError struct{}

func (m *MyServiceError) FuncError(ctx context.Context) error {
	return errors.New("TEST")
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
	_, err := new(Registry).exportedMethods(methodType)
	if err != nil {
		t.Error("Allows a private object argument to be exported")
	}

}

func getFunc1MethArgArray() []methodArgument {
	return []methodArgument{methodArgument{"int", reflect.TypeOf(0)},
		methodArgument{"string", reflect.TypeOf("")},
		methodArgument{"TestData", reflect.TypeOf(new(TestData))}}
}

func TestMethodArguments(t *testing.T) {
	objType := reflect.TypeOf(new(MyService))
	methodType := objType.Method(0).Type
	methodArgs, numPointers, err := new(Registry).searchMethodArguments(methodType)
	if err != nil {
		t.Error(err)
	}
	if 1 != numPointers {
		t.Error("Pointer counter fails to count")
	}
	expected := getFunc1MethArgArray()
	if !reflect.DeepEqual(methodArgs, expected) {
		t.Error("Got method arguments different from expected")
	}
}

func TestExportedMethods(t *testing.T) {
	methods, err := new(Registry).exportedMethods(reflect.TypeOf(new(MyService)))
	if err != nil {
		t.Error("Valid service does have an error in its methods")
	}
	expected := make(map[string]*methodData)
	expected["Func1"] = &methodData{
		method:      reflect.TypeOf(new(MyService)).Method(0),
		args:        getFunc1MethArgArray(),
		numPointers: 1}
	if !reflect.DeepEqual(methods, expected) {
		t.Error("Didn't get any method as exportable")
	}

	checkExportedFails(new(privateStuff), t)
	checkExportedFails(new(MyInvalidService1), t)
	checkExportedFails(new(MyInvalidService2), t)
	checkExportedFails(new(MyInvalidService3), t)
}

func TestRegister(t *testing.T) {
	registry := new(Registry)
	mysp := new(MyService)

	err := registry.Register(MyService{})
	if err == nil {
		t.Error("MyService without methods can be registerd (is not pointer):" + err.Error())
	}

	err = registry.RegisterWithName(mysp, "MS")
	if err != nil {
		t.Error("Could not register MyService:" + err.Error())
	}
	expectedMethods := make(map[string]*methodData)
	expectedMethods["Func1"] = &methodData{
		method:      reflect.TypeOf(mysp).Method(0),
		args:        getFunc1MethArgArray(),
		numPointers: 1}
	expected := make(map[string]*serviceData)
	expected["MS"] = &serviceData{
		name:    "MS",
		rcvr:    reflect.ValueOf(mysp),
		typ:     reflect.TypeOf(mysp),
		methods: expectedMethods}
	if !reflect.DeepEqual(registry.svcMap, expected) {
		t.Error("After registration the map doesn't contain what is expected")
	}

	err = registry.Register(privateStuff{})
	if err == nil {
		t.Error("privateStuff can be registered!")
	}
	err = registry.Register(MyService{})
	if err == nil {
		t.Error("MyService without methods can be registered twice!")
	}
	err = registry.Register(MyInvalidService1{})
	if err == nil {
		t.Error("MyInvalidService1 without methods can be registered")
	}
}

func TestGetServiceMethod(t *testing.T) {
	registry := new(Registry)
	if err := registry.Register(new(MyService)); err != nil {
		t.Error("Could not register MyService:" + err.Error())
	}
	svcData, mData := registry.GetServiceMethod("MyService", "Func1")
	if svcData == nil {
		t.Error("Didn't get MyService")
	}
	if mData == nil {
		t.Error("Didn't get Func1")
	}
}

func TestCall(t *testing.T) {
	registry := new(Registry)
	mysp := new(MyService)
	myerr := new(MyServiceError)
	if err := registry.Register(mysp); err != nil {
		t.Error("Could not register MyService:" + err.Error())
	}
	if err := registry.Register(myerr); err != nil {
		t.Error("Could not register MyServiceError:" + err.Error())
	}
	td := new(TestData)
	svcData, mData := registry.GetServiceMethod("MyService", "Func1")
	args := []reflect.Value{reflect.ValueOf(1), reflect.ValueOf("a"), reflect.ValueOf(td)}
	ctx := context.Background()
	svcData.ExecuteMethod(mData, ctx, args, func(rargs []reflect.Value, errMsg string) {
		if len(rargs) != 1 {
			t.Error("Returned args is different than 1 (" + strconv.Itoa(len(rargs)) + ")")
		}
		if rargs[0].Kind() != reflect.Ptr {
			t.Error("Returned arg is not a pointer")
		}
		td2 := rargs[0].Interface().(*TestData)
		if td2.A != 1 {
			t.Error("Value didn't come out as expected")
		}
	})

	svcData, mData = registry.GetServiceMethod("MyServiceError", "FuncError")
	args = []reflect.Value{}
	svcData.ExecuteMethod(mData, ctx, args, func(rargs []reflect.Value, errMsg string) {
		if len(rargs) != 0 {
			t.Error("Returned args is different than 0 (" + strconv.Itoa(len(rargs)) + ")")
		}
		if errMsg != "TEST" {
			t.Error("Received error is different")
		}
	})
}
