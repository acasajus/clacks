package clacks

import (
	"errors"
	"log"
	"net"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
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
	args := []interface{}{Args{1, 2}, &Reply{1}}
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
	recovered := []interface{}{args2[0].Interface(), args2[1].Interface()}
	if !reflect.DeepEqual(recovered, args) {
		t.Error("Something is not the same")
	}

}

func TestSendResponse(t *testing.T) {
	server := new(Server)
	if err := server.Register(new(DummyService)); err != nil {
		t.Error(err)
	}
	codec := new(gobCodec)
	codec.SetRWC(&RWCMock{})
	req := &Request{Method: "DummyService.Sum", Seq: 123}
	args := []reflect.Value{reflect.ValueOf(&Reply{1})}
	err := server.sendResponse(req, codec, "", args)
	if err != nil {
		t.Error(err)
	}
	resp := new(Response)
	if err = codec.ReadResponseHeader(resp); err != nil {
		t.Error(err)
	}
	if resp.Error != "" {
		t.Error("Received error is something")
	}
	if resp.Method != req.Method || resp.Seq != req.Seq {
		t.Error("Either method or seq mismatch")
	}
	ifaces := make([]interface{}, len(args))
	err = codec.ReadBody(&ifaces)
	if err != nil {
		t.Error(err)
	}
	if reflect.DeepEqual([]interface{}{&Reply{1}}, ifaces) {
		t.Error("Something is not the same")
	}
}

func TestProcessOne(t *testing.T) {
	server := new(Server)
	if err := server.Register(new(DummyService)); err != nil {
		t.Error(err)
	}
	codec := new(gobCodec)
	codec.SetRWC(&RWCMock{})
	req := &Request{Method: "DummyService.Sum", Seq: 123}
	args := []interface{}{Args{1, 2}, &Reply{1}}
	if err := codec.WriteRequest(req, args); err != nil {
		t.Error(err)
	}
	alive := server.ProcessOne(codec)
	if !alive {
		t.Error("OOps.. it's not alive..")
	}
	time.Sleep(100 * time.Millisecond)
	resp := new(Response)
	err := codec.ReadResponseHeader(resp)
	if err != nil {
		t.Error(err)
	}
}

func TestRPC(t *testing.T) {
	startNewServer()
	client, err := Dial("tcp", serverAddr)
	if err != nil {
		t.Fatal("dialing", err)
	}
	defer client.Close()
	a1 := Args{1, 2}
	rep := &Reply{100}
	err = client.Call("DummyService.Sum", a1, rep)
	if err != nil {
		t.Fatal("Calling DummyService.Sum: ", err)
	}
	if rep.Num != 3 {
		t.Error("Sum does not match")
	}
	err = client.Call("DummyService.Error", a1, rep)
	if err == nil {
		t.Fatal("Calling DummyService.Error error result is nil")
	}
	if err.Error() != "Test Error" {
		t.Fatal("Expected \"Test error\" and got " + err.Error())
	}
	//Different number of args
	err = client.Call("DummyService.Error", a1)
	if err == nil {
		t.Fatal("Calling DummyService.Error error result is nil")
	}
	if err.Error() != "Mismatch in the number of arguments! Expected 2" {
		t.Fatal("Expected \"Mismatch in the number of arguments! Expected 2\" and got " + err.Error())
	}
	//Different type of args
	err = client.Call("DummyService.Error", a1, a1)
	if err == nil {
		t.Fatal("Calling DummyService.Error error result is nil")
	}
	if err.Error() != "Argument 1 if of type clacks.Args and the expected type is *clacks.Reply" {
		t.Fatal("Expected \"Argument 1 if of type clacks.Args and the expected type is *clacks.Reply\" and got " + err.Error())
	}

	//Non existant func
	err = client.Call("DummyService.OOps", a1)
	if err == nil {
		t.Fatal("Calling DummyService.Oops error result is nil")
	}
	if err.Error() != "Can't find method OOps for service DummyService" {
		t.Fatal("Different expected error. Got", err)
	}
	//NOn existant svc
	err = client.Call("Nops.OOps", a1)
	if err == nil {
		t.Fatal("Calling DummyService.Oops error result is nil")
	}
	if err.Error() != "Can't find service Nops" {
		t.Fatal("Different expected error. Got", err)
	}
}
