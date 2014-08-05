package clacks

import (
	"errors"
	"fmt"

	"reflect"
	"sync"
)

type callback struct {
	method reflect.Value
	mid    uint64
}

type CallbackId struct {
	argName string
	mid     uint64
}

type CallbackManager struct {
	pushMap    map[string][]*callback
	midCounter uint64
	lock       sync.RWMutex
}

//Subscribe to pushed messages from the server
func (cbmgr *CallbackManager) Subscribe(cb interface{}) (CallbackId, error) {
	cbmgr.lock.Lock()
	defer cbmgr.lock.Unlock()
	sid := CallbackId{}
	mval := reflect.ValueOf(cb)
	mtype := mval.Type()
	if mtype.Kind() != reflect.Func {
		return sid, errors.New(fmt.Sprintf("%v is not a function", cb))
	}
	if mtype.NumIn() != 1 {
		return sid, errors.New(fmt.Sprintf("%v can only have one argument", cb))
	}
	arg := mtype.In(0)
	if arg.Kind() == reflect.Ptr {
		return sid, errors.New(fmt.Sprintf("%v cannot receive a pointer"))
	}
	if cbmgr.pushMap == nil {
		cbmgr.pushMap = make(map[string][]*callback)
	}
	argName := arg.String()
	ps := new(callback)
	ps.method = mval
	ps.mid = cbmgr.midCounter
	cbmgr.midCounter++
	sid.mid = ps.mid
	sid.argName = argName
	var subs []*callback
	var ok bool
	if subs, ok = cbmgr.pushMap[argName]; !ok {
		subs = make([]*callback, 0, 1)
	}
	cbmgr.pushMap[argName] = append(subs, ps)
	return sid, nil
}

//Unsubscribe to pushed messages from the server
func (cbmgr *CallbackManager) Unsubscribe(sid CallbackId) {
	cbmgr.lock.Lock()
	defer cbmgr.lock.Unlock()
	var subs []*callback
	var ok bool
	if subs, ok = cbmgr.pushMap[sid.argName]; !ok {
		return
	}
	for iPos, sub := range subs {
		if sub.mid == sid.mid {
			copy(subs[iPos:], subs[iPos+1:])
			subs = subs[:len(subs)-1]
			cbmgr.pushMap[sid.argName] = subs
		}
	}
}

//Execute all subscribed functions to a push message
func (cbmgr *CallbackManager) SendToAll(arg interface{}) {
	cbmgr.lock.RLock()
	defer cbmgr.lock.RUnlock()
	argType := reflect.TypeOf(arg)
	typeName := argType.String()
	isPointer := argType.Kind() == reflect.Ptr
	if isPointer {
		typeName = argType.Elem().String()
	}
	if subs, ok := cbmgr.pushMap[typeName]; ok {
		argVal := reflect.ValueOf(arg)
		for _, sub := range subs {
			var args []reflect.Value
			switch {
			case !isPointer:
				//Same received and expeted types
				args = []reflect.Value{argVal}
			case isPointer:
				//Received is pointer and expected is value
				args = []reflect.Value{argVal.Elem()}
			}
			//Execute the push in a goroutine
			go func(sub *callback, args []reflect.Value) {
				sub.method.Call(args)
			}(sub, args)
		}
	}
}