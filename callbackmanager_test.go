package clacks

import (
	"sync"
	"testing"
)

type PushData struct {
	A uint
	B uint
}

type Subs struct {
	Count uint
	Total uint
	WG    *sync.WaitGroup
}

func (s *Subs) inv1(pd PushData, a int) {
	s.doSomething(pd)
}

func (s *Subs) doSomethingPtr(pd *PushData) {
	s.doSomething(*pd)
}

func (s *Subs) doSomething(pd PushData) {
	s.Count++
	s.Total += pd.A + pd.B
	s.WG.Done()
}

func (s *Subs) doSomething2(pd PushData) {
	s.doSomething(pd)
}

func TestSubscribe(t *testing.T) {
	var sid1, sid2 CallbackId
	var err error
	cbmgr := new(CallbackManager)
	s := new(Subs)
	if _, err = cbmgr.Subscribe(Subs{}); err == nil {
		t.Error("Allow register of something that is not a function")
	}
	if _, err = cbmgr.Subscribe(s.inv1); err == nil {
		t.Error("Allow register of something that has more than one argument")
	}
	if _, err = cbmgr.Subscribe(s.doSomethingPtr); err == nil {
		t.Error("Allow register of something that has more than one argument")
	}
	if sid1, err = cbmgr.Subscribe(s.doSomething); err != nil {
		t.Error(err)
	}
	subs := cbmgr.pushMap["clacks.PushData"]
	if len(subs) != 1 {
		t.Fatal("Did not register")
	}
	if subs[0].mid != sid1.mid {
		t.Error("mids differ")
	}
	if sid2, err = cbmgr.Subscribe(s.doSomething2); err != nil {
		t.Error(err)
	}
	subs = cbmgr.pushMap["clacks.PushData"]
	if len(subs) != 2 {
		t.Fatal("Did not register")
	}
	if subs[1].mid != sid2.mid {
		t.Error("mids differ")
	}
	//Try unsubscribe
	cbmgr.Unsubscribe(sid2)
	subs = cbmgr.pushMap["clacks.PushData"]
	if len(subs) != 1 {
		t.Fatal("Did not unregister")
	}
	if subs[0].mid != sid1.mid {
		t.Error("Didn't delete what was expected")
	}
	cbmgr.Unsubscribe(sid1)
	subs = cbmgr.pushMap["clacks.PushData"]
	if len(subs) != 0 {
		t.Fatal("Did not unregister")
	}
}

func TestPushCB(t *testing.T) {
	cbmgr := new(CallbackManager)
	s := new(Subs)
	s.WG = new(sync.WaitGroup)
	if _, err := cbmgr.Subscribe(s.doSomething); err != nil {
		t.Error(err)
	}
	if _, err := cbmgr.Subscribe(s.doSomething2); err != nil {
		t.Error(err)
	}
	s.WG.Add(2)
	cbmgr.SendToAll(&PushData{1, 2})
	s.WG.Wait()
	if s.Total != 6 || s.Count != 2 {
		t.Error("Something didn't go as expected")
	}
	s.WG.Add(2)
	cbmgr.SendToAll(PushData{1, 2})
	s.WG.Wait()
	if s.Total != 12 || s.Count != 4 {
		t.Error("Something didn't go as expected")
	}
}
