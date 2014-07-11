package clacks

import (
	"errors"
	"log"
	"reflect"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Precompute the reflect type for error.  Can't use error directly
// because Typeof takes an empty interface value.  This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

type methodData struct {
	sync.Mutex  // protects counters
	method      reflect.Method
	ArgsType    []reflect.Type
	RepliesType []reflect.Type
	numCalls    uint
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

func methodArguments(getter func(int) reflect.Type, totalElements int) ([]reflect.Type, error) {
	exported := make([]reflect.Type, 0)
	for i := 0; i < totalElements; i++ {
		argType := getter(i)
		if argType.Kind() == reflect.Ptr {
			argType = argType.Elem()
		}
		if !isExportedOrBuiltinType(argType) {
			return exported, errors.New("argument type not exported:" + argType.String())
		}
		exported = append(exported, argType)
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
		methodArgs, err := methodArguments(methodType.In, methodType.NumIn())
		if err != nil {
			return methods, errors.New(methodName + " has an invalid argument: " + err.Error())
		}
		methodReplies, err := methodArguments(methodType.Out, methodType.NumOut()-1)
		if err != nil {
			return methods, errors.New(methodName + " has an invalid return: " + err.Error())
		}
		// The return type of the method must be error.
		if returnType := methodType.Out(methodType.NumOut() - 1); returnType != typeOfError {
			return methods, errors.New("method" + methodName + "returns" + returnType.String() + "not error as last return value")
		}
		methods[methodName] = &methodData{method: method, ArgsType: methodArgs, RepliesType: methodReplies}
	}
	return methods, nil
}

func (router *Router) register(rcvr interface{}) error {
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
		log.Print(s)
		return errors.New(s)
	}
	if _, present := router.svcMap[sname]; present {
		return errors.New("rpc: service already defined: " + sname)
	}
	s.name = sname

	// Install the methods
	methods, _ := exportedMethods(s.typ)
	s.methods = methods

	if len(s.methods) == 0 {
		str := ""

		// To help the user, see if a pointer
		// receiver would work.
		method, _ := exportedMethods(reflect.PtrTo(s.typ))
		if len(method) != 0 {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type"
		}
		log.Print(str)
		return errors.New(str)
	}
	router.svcMap[s.name] = s
	return nil
}