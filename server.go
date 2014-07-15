package clacks

import (
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

const (
	connectedMsg = "200 RPC"
	RpcPath      = "/RPC"
)

type Request struct {
	Seq  uint64
	next *Request
}

type Response struct {
	Seq  uint64
	next *Response
}

type Server struct {
	creationLock sync.Mutex
	registry     *Registry
	reqLock      sync.Mutex // protects freeReq
	freeReq      *Request
	respLock     sync.Mutex // protects freeResp
	freeResp     *Response
}

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
func (server *Server) ProcessConnection(conn io.ReadWriteCloser) {

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
	//server.ServeConn(conn)
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