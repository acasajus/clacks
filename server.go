package clacks

import (
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"code.google.com/p/go.net/context"
)

const (
	connectedMsg = "200 HIJACK"
	RPCPath      = "/RPC"
)

type codecFunc func(io.ReadWriteCloser) Codec
type contextFunc func(context.Context, net.Conn) context.Context

type Server struct {
	ReCache
	creationLock sync.Mutex
	registry     *Registry
	codecCB      codecFunc
	contextCB    contextFunc
}

/* Generate codec */

func GenerateCodec(conn io.ReadWriteCloser) Codec {
	codec := &gobCodec{}
	codec.SetRWC(conn)
	return codec
}

/* Methods to set callbacks by user */

func (server *Server) CodecFunc(c codecFunc) {
	server.codecCB = c
}

func (server *Server) ContextFunc(c contextFunc) {
	server.contextCB = c
}

/*
Process
*/

func (server *Server) ProcessConnection(conn net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if server.contextCB != nil {
		ctx = server.contextCB(ctx, conn)
	}
	codec := server.codecCB(conn)
	defer codec.Close()
	for server.processOne(ctx, codec) {
	}
}

func (server *Server) processOne(ctx context.Context, codec Codec) bool {
	req, alive, svc, mData, args, err := server.readRequest(codec)
	if err != nil {
		if !alive {
			return false
		}
		// send a response if we actually managed to read a header.
		if req != nil {
			server.sendResponse(req, codec, err.Error(), nil)
		}
	} else {
		go svc.ExecuteMethod(mData, ctx, args, func(rargs []reflect.Value, errMsg string) {
			server.sendResponse(req, codec, errMsg, rargs)
		})
	}
	return true
}

func (server *Server) sendResponse(req *Request, codec Codec, errMsg string, rargs []reflect.Value) (err error) {
	resp := server.getResponse()
	defer server.freeRequest(req)
	defer server.freeResponse(resp)
	resp.Seq = req.Seq
	resp.Error = errMsg
	if len(resp.Error) > 0 {
		err = codec.WriteResponse(resp, nil)
	} else {
		ifaces := make([]interface{}, len(rargs))
		for iPos, argv := range rargs {
			ifaces[iPos] = argv.Interface()
		}
		err = codec.WriteResponse(resp, ifaces)
	}
	if err != nil {
		log.Println("writing response:", err)
	}
	return
}

func (server *Server) readRequest(codec Codec) (req *Request, alive bool, svcData *serviceData, mData *methodData, args []reflect.Value, err error) {
	req, alive, svcData, mData, err = server.readRequestHeader(codec)
	if err != nil {
		if alive {
			codec.ReadBody(nil)
		}
		log.Println("Error processing read request header: ", err)
		return
	}
	//Fill the interface array with the expected types
	ifaces := make([]interface{}, 0)
	err = codec.ReadBody(&ifaces)
	if err != nil {
		return
	}
	numArgs := len(mData.args)
	if len(ifaces) != numArgs {
		err = errors.New("Mismatch in the number of arguments! Expected " + strconv.Itoa(numArgs))
		return
	}
	args = make([]reflect.Value, numArgs)
	for iPos, _ := range mData.args {
		args[iPos] = reflect.ValueOf(ifaces[iPos]).Elem()
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
	http.Handle(RPCPath, server)
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
	return &Server{codecCB: GenerateCodec}
}
