package clacks

import (
	"errors"
	"log"
	"net"
	"net/http/httptest"
	"reflect"
	"testing"
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
	Num int
}

type DummyService struct{}

func (ds *DummyService) Sum(a Args, r *Reply) error {
	r.Num = a.A + a.B
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
func TestReadRequestHeader(t *testing.T) {
	server := new(Server)
	if err := server.Register(new(DummyService)); err != nil {
		t.Error(err)
	}
	codec := new(gobCodec)
	codec.SetRWC(&RWCMock{})
	req := &Request{Method: "DummyService.Sum", Seq: 123}
	if err := codec.WriteRequest(req, struct{}{}); err != nil {
		t.Error(err)
	}
	req2, alive, svcData, mData, err := server.readRequestHeader(codec)
	if !reflect.DeepEqual(req, req2) {
		t.Error("Request is not the same")
	}
	if !alive {
		t.Error("It says it's not alive")
	}
	if svcData.name != "DummyService" {
		t.Error("Service is not the same")
	}
	if mData.method.Name != "Sum" {
		t.Error("Method is not the expected one")
	}
	if err != nil {
		t.Error(err)
	}
	//Malformed method
	req.Method = "ASD"
	if err := codec.WriteRequest(req, struct{}{}); err != nil {
		t.Error(err)
	}
	req2, alive, svcData, mData, err = server.readRequestHeader(codec)
	if err == nil {
		t.Error("Should get error for malformed method")
	}
	//Invalid service
	req.Method = "ASD.typeOfErrorASD"
	if err := codec.WriteRequest(req, struct{}{}); err != nil {
		t.Error(err)
	}
	req2, alive, svcData, mData, err = server.readRequestHeader(codec)
	if err == nil {
		t.Error("Should get error for invalid service")
	}
	//Invalid service
	req.Method = "DummyService.OOPS"
	if err := codec.WriteRequest(req, struct{}{}); err != nil {
		t.Error(err)
	}
	req2, alive, svcData, mData, err = server.readRequestHeader(codec)
	if err == nil {
		t.Error("Should get error for invalid method")
	}
}

func TestReadRequest(t *testing.T) {
	server := new(Server)
	if err := server.Register(new(DummyService)); err != nil {
		t.Error(err)
	}
	codec := new(gobCodec)
	codec.SetRWC(&RWCMock{})
	req := &Request{Method: "DummyService.Sum", Seq: 123}
	args := []interface{}{Args{1, 2}, &Reply{}}
	if err := codec.WriteRequest(req, args); err != nil {
		t.Error(err)
	}
	req2, alive, svcData, mData, args2, err := server.readRequest(codec)
	if !reflect.DeepEqual(req, req2) {
		t.Error("Request is not the same")
	}
	if !alive {
		t.Error("It says it's not alive")
	}
	if svcData.name != "DummyService" {
		t.Error("Service is not the same")
	}
	if mData.method.Name != "Sum" {
		t.Error("Method is not the expected one")
	}
	if err != nil {
		t.Error(err)
	}
	if len(args2) != 2 {
		t.Error("Processed args should be length 2")
	}
	expected := []reflect.Value{reflect.ValueOf(args[0]), reflect.ValueOf(args[1])}
	if !reflect.DeepEqual(expected, args2) {
		t.Error("Something is not the same")
	}

}
