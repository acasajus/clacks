package clacks

import (
	"net"

	"code.google.com/p/go.net/context"
)

type contextKey int

const (
	connIdKey = iota
	connKey
)

type Context struct {
	ctx context.Context
}

func NewContext() *Context {
	return &Context{context.Background()}
}

//Get a Cancellation function for this context
func (me *Context) GetCancelFunc() context.CancelFunc {
	var cancel context.CancelFunc
	me.ctx, cancel = context.WithCancel(me.ctx)
	return cancel
}

func (me *Context) Done() <-chan struct{} {
	return me.ctx.Done()
}

//Set a value for a key
func (me *Context) SetValue(key interface{}, value interface{}) {
	me.ctx = context.WithValue(me.ctx, key, value)
}

//Retrieve the value for a key
func (me *Context) GetValue(key interface{}) interface{} {
	return me.ctx.Value(key)
}

func (me *Context) setClientId(connId uint64) {
	me.ctx = context.WithValue(me.ctx, connIdKey, connId)
}

//Get the client id
func (me *Context) GetClientId() uint64 {
	return me.ctx.Value(connIdKey).(uint64)
}

func (me *Context) setConn(conn net.Conn) {
	me.ctx = context.WithValue(me.ctx, connKey, conn)
}

func (me *Context) getConn() net.Conn {
	return me.ctx.Value(connKey).(net.Conn)
}

//Get client IP from context
func (me *Context) GetClientAddr() net.Addr {
	return me.getConn().RemoteAddr()
}