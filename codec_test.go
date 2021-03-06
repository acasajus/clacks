package clacks

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

type BodyData struct {
	A int
	B string
}

/* START OF MOCK */

type RWCMock struct {
	data   bytes.Buffer
	closed bool
}

func (rwc *RWCMock) Write(data []byte) (n int, err error) {
	if rwc.closed {
		return 0, io.ErrUnexpectedEOF
	}
	return rwc.data.Write(data)
}

func (rwc *RWCMock) Read(data []byte) (n int, err error) {
	if rwc.closed {
		return 0, io.EOF
	}
	return rwc.data.Read(data)
}

func (rwc *RWCMock) Close() error {
	rwc.closed = true
	return nil
}

/* END OF MOCK */

func TestGobCodec(t *testing.T) {
	buf := RWCMock{}
	codec := &gobCodec{}
	codec.SetRWC(&buf)
	req := Request{Seq: 3}
	resp := Response{Seq: 9}
	data := BodyData{234234, "LOL"}

	if err := codec.WriteRequest(&req, data); err != nil {
		t.Error(err)
	}
	if err := codec.WriteResponse(&resp, data); err != nil {
		t.Error(err)
	}

	readReq := new(Request)
	if err := codec.ReadRequestHeader(readReq); err != nil {
		t.Error(err)
	}
	readBody := new(BodyData)
	if err := codec.ReadBody(readBody); err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(req, *readReq) {
		t.Error("Requests are not the same")
	}
	if !reflect.DeepEqual(data, *readBody) {
		t.Error("Request bodies are not the same")
	}

	readResp := new(Response)
	if err := codec.ReadResponseHeader(readResp); err != nil {
		t.Error(err)
	}
	if err := codec.ReadBody(readBody); err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(resp, *readResp) {
		t.Error("Response are not the same")
	}
	if !reflect.DeepEqual(data, *readBody) {
		t.Error("Response bodies are not the same")
	}

}
