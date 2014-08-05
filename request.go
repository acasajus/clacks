package clacks

import "sync"

const (
	R_RPC  = iota //Normal RPC request
	R_PUSH        //Push async data to client
	R_DATA        //Send data to client
)

type Request struct {
	Method string
	Seq    uint64
	next   *Request
}

type Response struct {
	Type  uint8
	Seq   uint64
	Error string
	next  *Response
}

type ReCache struct {
	reqLock  sync.Mutex // protects freeReq
	freeReq  *Request
	respLock sync.Mutex // protects freeResp
	freeResp *Response
}

/*
 Mem caching of Request and Response objects
*/

func (rc *ReCache) getRequest() *Request {
	rc.reqLock.Lock()
	defer rc.reqLock.Unlock()
	req := rc.freeReq
	if req == nil {
		req = new(Request)
	} else {
		rc.freeReq = req.next
		*req = Request{}
	}
	return req
}

func (rc *ReCache) freeRequest(req *Request) {
	rc.reqLock.Lock()
	defer rc.reqLock.Unlock()
	req.next = rc.freeReq
	rc.freeReq = req
}

func (rc *ReCache) getResponse() *Response {
	rc.respLock.Lock()
	defer rc.respLock.Unlock()
	resp := rc.freeResp
	if resp == nil {
		resp = new(Response)
	} else {
		rc.freeResp = resp.next
		*resp = Response{}
	}
	return resp
}

func (rc *ReCache) freeResponse(resp *Response) {
	rc.respLock.Lock()
	defer rc.respLock.Unlock()
	resp.next = rc.freeResp
	rc.freeResp = resp
}