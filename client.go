package clacks

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"sync"
)

// ServerError represents an error that has been returned from
// the remote side of the RPC connection.
type ServerError string

func (e ServerError) Error() string {
	return string(e)
}

var ErrShutdown = errors.New("connection is shut down")

type Client struct {
	codec Codec
	cbmgr *CallbackManager

	sending sync.Mutex

	mutex    sync.Mutex // protects following
	request  Request
	seq      uint64
	pending  map[uint64]*Call
	closing  bool // user has called Close
	shutdown bool // server has told us to stop
}

type Call struct {
	Method string        // The name of the service and method to call.
	Args   []interface{} // The argument to the function (*struct).
	Error  error         // After completion, the error status.
	Done   chan *Call    // Strobes when call is complete.
}

type disconnectType *Client

func (call *Call) done() {
	select {
	case call.Done <- call:
		// ok
	default:
		// We don't want to block here.  It is the caller's responsibility to make
		// sure the channel has enough buffer space. See comment in Go().
		//if debugLog {
		//	log.Println("rpc: discarding Call reply due to insufficient Done chan capacity")
		//}
	}
}

func (client *Client) readResponseBody(call *Call) error {
	ifaces := make([]interface{}, 0)
	err := client.codec.ReadBody(&ifaces)
	if err != nil {
		return err
	}
	replyPos := 0
	for _, arg := range call.Args {
		if reflect.ValueOf(arg).Kind() == reflect.Ptr {
			if replyPos >= len(ifaces) {
				return errors.New("Return data did not include all pointer values")
			}
			rplVal := reflect.ValueOf(ifaces[replyPos])
			if rplVal.Kind() != reflect.Ptr || rplVal.Elem().Kind() != reflect.Ptr {
				return errors.New("Return position " + strconv.Itoa(replyPos) + " is not a **Type")
			}
			replyPos++
			reflect.ValueOf(arg).Elem().Set(rplVal.Elem().Elem())
		}
	}
	if replyPos < len(ifaces) {
		return errors.New("Return data has more pointer values than expected")
	}
	return err
}

func (client *Client) processRPCResponse(response Response) (err error) {
	seq := response.Seq
	client.mutex.Lock()
	call := client.pending[seq]
	delete(client.pending, seq)
	client.mutex.Unlock()

	switch {
	case call == nil:
		// We've got no pending call. That usually means that
		// WriteRequest partially failed, and call was already
		// removed; response is a server telling us about an
		// error reading request body
		if response.Error != "" {
			err = errors.New(response.Error)
		}
	case response.Error != "":
		// We've got an error response. Give this to the request;
		// any subsequent requests will get the ReadResponseBody
		// error if there is one.
		call.Error = ServerError(response.Error)
		call.done()
	default:
		err = client.readResponseBody(call)
		if err != nil {
			call.Error = errors.New("reading body " + err.Error())
		}
		call.done()
	}
	return
}

func (client *Client) processPushResponse(response Response) (err error) {
	var data interface{}
	err = client.codec.ReadBody(&data)
	if err != nil {
		client.cbmgr.SendToAll(data)
	}
	return
}

func (client *Client) processInput() {
	var err error
	var response Response
	for err == nil {
		response = Response{}
		err = client.codec.ReadResponseHeader(&response)
		if err != nil {
			break
		}
		switch response.Type {
		case R_RPC:
			err = client.processRPCResponse(response)
		case R_PUSH:
			err = client.processPushResponse(response)
		}

	}
	// Terminate pending calls.
	client.sending.Lock()
	client.mutex.Lock()
	client.shutdown = true
	closing := client.closing
	if err == io.EOF {
		if closing {
			err = ErrShutdown
		} else {
			err = io.ErrUnexpectedEOF
		}
	}
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
	client.mutex.Unlock()
	client.sending.Unlock()
	var disc disconnectType
	disc = client
	client.cbmgr.SendToAll(disc)
	//if debugLog && err != io.EOF && !closing {
	//	log.Println("rpc: client protocol error:", err)
	//}
}

func (client *Client) Close() error {
	client.mutex.Lock()
	if client.closing {
		client.mutex.Unlock()
		return ErrShutdown
	}
	client.closing = true
	client.mutex.Unlock()
	return client.codec.Close()
}

func (client *Client) SubscribeToDisconnect(cb func(*Client)) error {
	_, err := client.cbmgr.Subscribe(func(disc disconnectType) {
		cb(disc)
	})
	return err
}

func (client *Client) SubscribeToPush(cb interface{}) error {
	_, err := client.cbmgr.Subscribe(cb)
	return err
}

// Go invokes the function asynchronously.  It returns the Call structure representing
// the invocation.  The done channel will signal when the call is complete by returning
// the same Call object.  If done is nil, Go will allocate a new channel.
// If non-nil, done must be buffered or Go will deliberately crash.
func (client *Client) Go(done chan *Call, serviceMethod string, args ...interface{}) *Call {
	call := new(Call)
	call.Method = serviceMethod
	call.Args = args
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel.  If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("done channel is unbuffered")
		}
	}
	call.Done = done
	client.send(call)
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(serviceMethod string, args ...interface{}) error {
	call := <-client.Go(make(chan *Call, 1), serviceMethod, args...).Done
	return call.Error
}

/* Dial methods */

// DialHTTP connects to an HTTP RPC server at the specified network address
// listening on the default HTTP RPC path.
func DialHTTP(network, address string) (*Client, error) {
	return DialHTTPPath(network, address, RPCPath)
}

// DialHTTPPath connects to an HTTP RPC server
// at the specified network address and path.
func DialHTTPPath(network, address, path string) (*Client, error) {
	var err error
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	io.WriteString(conn, "CONNECT "+path+" HTTP/1.0\n\n")

	// Require successful HTTP response
	// before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connectedMsg {
		return NewClient(conn), nil
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	conn.Close()
	return nil, &net.OpError{
		Op:   "dial-http",
		Net:  network + " " + address,
		Addr: nil,
		Err:  err,
	}
}

func Dial(network string, address string) (*Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}

/* New client methods */

// It adds a buffer to the write side of the connection so
// the header and payload are sent as a unit.
func NewClient(conn io.ReadWriteCloser) *Client {
	codec := new(gobCodec)
	codec.SetRWC(conn)
	return NewClientWithCodec(codec)
}

func NewClientWithCodec(codec Codec) *Client {
	client := &Client{
		codec:   codec,
		pending: make(map[uint64]*Call),
		cbmgr:   new(CallbackManager),
	}
	go client.processInput()
	return client
}

func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	// Register this call.
	client.mutex.Lock()
	if client.shutdown || client.closing {
		call.Error = ErrShutdown
		client.mutex.Unlock()
		call.done()
		return
	}
	seq := client.seq
	client.seq++
	client.pending[seq] = call
	client.mutex.Unlock()

	// Encode and send the request.
	client.request.Seq = seq
	client.request.Method = call.Method
	err := client.codec.WriteRequest(&client.request, call.Args)
	if err != nil {
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}
