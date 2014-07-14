package clacks

import (
	"errors"

	"reflect"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Precompute the reflect type for error.  Can't use error directly
// because Typeof takes an empty interface value.  This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

type methodArgument struct {
	name    string
	typ     reflect.Type
	pointer bool
}

type methodData struct {
	sync.Mutex // protects counters
	method     reflect.Method
	args       []methodArgument
	numCalls   uint
}

type service struct {
	name    string                 // name of service
	rcvr    reflect.Value          // receiver of methods for the service
	typ     reflect.Type           // type of the receiver
	methods map[string]*methodData // registered methods
}

type Router struct {
	svcMap map[string]*service
	lock   sync.RWMutex
}

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

func searchMethodArguments(methodType reflect.Type) ([]methodArgument, error) {
	exported := make([]methodArgument, 0)
	//First In is the interfaced stuct itself
	for i := 1; i < methodType.NumIn(); i++ {
		argType := methodType.In(i)
		pointer := false
		if argType.Kind() == reflect.Ptr {
			argType = argType.Elem()
			pointer = true
		}
		mArg := methodArgument{pointer: pointer, name: argType.Name(), typ: argType}
		if !isExportedOrBuiltinType(argType) {
			return exported, errors.New("argument type not exported:" + argType.String())
		}
		exported = append(exported, mArg)
	}
	if cap(exported) > len(exported) {
		tmp := make([]methodArgument, len(exported))
		copy(tmp, exported)
		exported = tmp
	}
	return exported, nil
}

// exportedMethods returns suitable Rpc methods of typ, it will report
// error using log if reportErr is true.
func exportedMethods(typ reflect.Type) (map[string]*methodData, error) {
	methods := make(map[string]*methodData)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		methodType := method.Type
		methodName := method.Name
		// Method must be exported.
		if method.PkgPath != "" || !isExported(methodName) {
			continue
		}
		methodArgs, err := searchMethodArguments(methodType)
		if err != nil {
			return methods, errors.New(methodName + " has an invalid argument: " + err.Error())
		}
		if methodType.NumOut() != 1 {
			return methods, errors.New(methodName + " can only return one value and it has to be an error")
		}
		// The return type of the method must be error.
		if returnType := methodType.Out(methodType.NumOut() - 1); returnType != typeOfError {
			return methods, errors.New("method" + methodName + "returns" + returnType.String() + "not error as last return value")
		}
		methods[methodName] = &methodData{method: method, args: methodArgs}
	}
	return methods, nil
}

func (router *Router) Register(rcvr interface{}) error {
	router.lock.Lock()
	defer router.lock.Unlock()
	if router.svcMap == nil {
		router.svcMap = make(map[string]*service)
	}
	s := new(service)
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	sname := reflect.Indirect(s.rcvr).Type().Name()
	if !isExported(sname) {
		s := "rpc.Register: type " + sname + " is not exported"
		return errors.New(s)
	}
	if _, present := router.svcMap[sname]; present {
		return errors.New("rpc: service already defined: " + sname)
	}
	s.name = sname

	// Install the methods
	methods, err := exportedMethods(s.typ)
	if err != nil {
		return errors.New(sname + "cannot be registered: " + err.Error())
	}
	s.methods = methods

	if len(s.methods) == 0 {
		// To help the user, see if a pointer
		// receiver would work.
		methods, err := exportedMethods(reflect.PtrTo(s.typ))
		switch {
		case len(methods) != 0:
			return errors.New("Type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)")
		case err != nil:
			return err
		}
		return errors.New("Type " + sname + " has no exported methods of suitable type")
	}
	router.svcMap[s.name] = s
	return nil
}