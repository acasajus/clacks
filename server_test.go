package clacks

import (
	"errors"
	"log"
	"net"
	"net/http/httptest"
)

//Test helpers

var (
	server     *Server
	serverAddr string
)

type Args struct {
	A, B int
}

type Reply struct {
	num int
}

type DummyService struct{}

func (ds *DummyService) Sum(a Args, r *Reply) error {
	r.num = a.A + a.B
	return nil
}

func (ds *DummyService) Error(a Args, r *Reply) error {
	return errors.New("Test Error")
}

func startNewServer() {
	server = NewServer()
	server.Register(new(DummyService))

	l, err := net.Listen("tcp", "127.0.0.1:0") // any available address
	if err != nil {
		log.Fatalf("net.Listen tcp :0: %v", err)
	}
	serverAddr = l.Addr().String()

	log.Println("NewServer test RPC server listening on", serverAddr)
	go server.Accept(l)

	server.HandleHTTP()
	httpAddr := httptest.NewServer(nil).Listener.Addr().String()
	log.Println("Test HTTP RPC server listening on", httpAddr)
}

// END HELPERS
