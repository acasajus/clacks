package clacks

import (
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

const (
	connectedMsg = "200 HIJACK"
	RpcPath      = "/RPC"
)

type Request struct {
	Method string
	Seq    uint64
	next   *Request
}

type Response struct {
	Method string
	Seq    uint64
	Error  string
	next   *Response
}

type Server struct {
	creationLock sync.Mutex
	registry     *Registry
	reqLock      sync.Mutex // protects freeReq
	freeReq      *Request
	respLock     sync.Mutex // protects freeResp
	freeResp     *Response
}

var invalidRequest = []reflect.Value{reflect.ValueOf(struct{}{})}

/*
 Mem caching of Request and Response objects
*/

func (server *Server) getRequest() *Request {
	server.reqLock.Lock()
	defer server.reqLock.Unlock()
	req := server.freeReq
	if req == nil {
		req = new(Request)
	} else {
		server.freeReq = req.next
		*req = Request{}
	}
	return req
}

func (server *Server) freeRequest(req *Request) {
	server.reqLock.Lock()
	defer server.reqLock.Unlock()
	req.next = server.freeReq
	server.freeReq = req
}

func (server *Server) getResponse() *Response {
	server.respLock.Lock()
	defer server.respLock.Unlock()
	resp := server.freeResp
	if resp == nil {
		resp = new(Response)
	} else {
		server.freeResp = resp.next
		*resp = Response{}
	}
	return resp
}

func (server *Server) freeResponse(resp *Response) {
	server.respLock.Lock()
	defer server.respLock.Unlock()
	resp.next = server.freeResp
	server.freeResp = resp
}

/*
Process
*/

//To use a different codec "overload" this method and execute ProcessCodec with your own Codec
func (server *Server) ProcessConnection(conn io.ReadWriteCloser) {
	codec := &gobCodec{}
	codec.SetRWC(conn)
	server.ProcessCodec(codec)
}

func (server *Server) ProcessCodec(codec Codec) {
	defer codec.Close()
	for {
		req, alive, svc, mData, args, err := server.readRequest(codec)
		if err != nil {
			log.Println(err)
			if !alive {
				break
			}
			// send a response if we actually managed to read a header.
			if req != nil {
				server.sendResponse(req, codec, invalidRequest, err.Error())
			}
			continue
		}
		go svc.executeMethod(mData, args, func(rargs []reflect.Value, errMsg string) {
			server.sendResponse(req, codec, rargs, errMsg)
		})
	}
}

func (server *Server) sendResponse(req *Request, codec Codec, rargs []reflect.Value, errMsg string) {
	resp := server.getResponse()
	defer server.freeRequest(req)
	defer server.freeResponse(resp)
	resp.Method = req.Method
	resp.Seq = req.Seq
	resp.Error = errMsg
	err := codec.WriteResponse(resp, rargs)
	if err != nil {
		log.Println("writing response:", err)
	}
}

func (server *Server) readRequest(codec Codec) (req *Request, alive bool, svcData *serviceData, mData *methodData, args []reflect.Value, err error) {
	req, alive, svcData, mData, err = server.readRequestHeader(codec)
	if err != nil {
		if !alive {
			codec.ReadBody(nil)
		}
		return
	}
	ifaces := make([]interface{}, len(mData.args))
	codec.ReadBody(ifaces)
	args = make([]reflect.Value, len(mData.args))
	for iPos, methodArg := range mData.args {
		if methodArg.typ.Kind() == reflect.Ptr {
			args[iPos] = reflect.New(methodArg.typ.Elem())
		} else {
			args[iPos] = reflect.New(methodArg.typ)
		}
		args[iPos] = reflect.ValueOf(ifaces[iPos])
	}
	for iPos, methodArg := range mData.args {
		if methodArg.typ.Kind() != reflect.Ptr {
			args[iPos] = args[iPos].Elem()
		}
	}
	return

}

func (server *Server) readRequestHeader(codec Codec) (req *Request, alive bool, svcData *serviceData, mData *methodData, err error) {
	req = server.getRequest()
	err = codec.ReadRequestHeader(req)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = errors.New("server cannot decode request: " + err.Error())
		return
	}
	alive = true
	dot := strings.LastIndex(req.Method, ".")
	if dot < 0 {
		err = errors.New("service/method request ill-formed: " + req.Method)
		return
	}
	serviceName := req.Method[:dot]
	methodName := req.Method[dot+1:]

	svcData, mData = server.registry.GetServiceMethod(serviceName, methodName)
	if svcData == nil {
		err = errors.New("Can't find service " + serviceName)
		return
	}
	if mData == nil {
		err = errors.New("Can't find method " + methodName + " for service " + serviceName)
	}
	return
}

/*
	HTTP bridge
*/
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	io.WriteString(conn, "HTTP/1.0 "+connectedMsg+"\n\n")
	server.ProcessConnection(conn)
}

//This method will bind the different HTTP endpoints to their handlers
func (server *Server) HandleHTTP() {
	http.Handle(RpcPath, server)
}

func (server *Server) Register(endpoint interface{}) error {
	server.creationLock.Lock()
	if server.registry == nil {
		server.registry = new(Registry)
	}
	server.creationLock.Unlock()
	return server.registry.Register(endpoint)
}

// Accept accepts connections on the listener and serves requests
// for each incoming connection.  Accept blocks; the caller typically
// invokes it in a go statement.
func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Fatal("rpc.Serve: accept:", err.Error()) // TODO(r): exit?
		}
		go server.ProcessConnection(conn)
	}
}

func (server *Server) ListenAndServe(addr string) {
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}
func (server *Server) ListenAndServeTLS(addr string, certFile string, keyFile string) {
	err := http.ListenAndServeTLS(addr, certFile, keyFile, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func NewServer() *Server {
	return new(Server)
}