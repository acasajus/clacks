package clacks

import (
	"reflect"
	"testing"
)

func TestGetRequest(t *testing.T) {
	rc := new(ReCache)
	req := rc.getRequest()
	reqI := reflect.ValueOf(req).Pointer()
	req2 := rc.getRequest()
	req2I := reflect.ValueOf(req2).Pointer()
	rc.freeRequest(req)
	req3 := rc.getRequest()
	req3I := reflect.ValueOf(req3).Pointer()
	if reqI != req3I {
		t.Error("Request was not cached")
	}
	if req2I == req3I {
		t.Error("Request should be different")
	}

}

func TestGetResponse(t *testing.T) {
	rc := new(ReCache)
	req := rc.getResponse()
	reqI := reflect.ValueOf(req).Pointer()
	req2 := rc.getResponse()
	req2I := reflect.ValueOf(req2).Pointer()
	rc.freeResponse(req)
	req3 := rc.getResponse()
	req3I := reflect.ValueOf(req3).Pointer()
	if reqI != req3I {
		t.Error("Response was not cached")
	}
	if req2I == req3I {
		t.Error("Response should be different")
	}

}
