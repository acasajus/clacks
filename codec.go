package clacks

import (
	"bufio"
	"compress/flate"
	"encoding/gob"
	"io"
	"sync"
)

type Codec interface {
	Register(interface{})
	WriteRequest(*Request, interface{}) error
	WriteResponse(*Response, interface{}) error
	ReadRequestHeader(*Request) error
	ReadResponseHeader(*Response) error
	ReadBody(interface{}) error
	Close() error
}

type gobCodec struct {
	rwc       io.ReadWriteCloser
	dec       *gob.Decoder
	enc       *gob.Encoder
	zip       *flate.Writer
	encBuf    *bufio.Writer
	writeLock sync.Mutex
}

func (c *gobCodec) Register(val interface{}) {
	gob.Register(val)
}

func (c *gobCodec) SetRWC(rwc io.ReadWriteCloser) {
	c.rwc = rwc
	c.zip, _ = flate.NewWriter(rwc, 9)
	c.encBuf = bufio.NewWriter(c.zip)
	c.dec = gob.NewDecoder(flate.NewReader(rwc))
	c.enc = gob.NewEncoder(c.encBuf)
}

func (c *gobCodec) flush() error {
	err := c.encBuf.Flush()
	if err != nil {
		return err
	}
	return c.zip.Flush()
}

func (c *gobCodec) WriteRequest(r *Request, body interface{}) (err error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	if err = c.enc.Encode(r); err != nil {
		return
	}
	if body != nil {
		if err = c.enc.Encode(body); err != nil {
			return
		}
	}
	return c.flush()
}

func (c *gobCodec) WriteResponse(r *Response, body interface{}) (err error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	if err = c.enc.Encode(r); err != nil {
		return
	}
	if body != nil {
		if err = c.enc.Encode(body); err != nil {
			return
		}
	}
	return c.flush()
}

func (c *gobCodec) ReadRequestHeader(r *Request) error {
	return c.dec.Decode(r)
}

func (c *gobCodec) ReadResponseHeader(r *Response) error {
	return c.dec.Decode(r)
}

func (c *gobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *gobCodec) Close() error {
	return c.rwc.Close()
}