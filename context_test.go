package clacks

import "testing"

/* TEST START */

func TestCancel(t *testing.T) {
	ctx := NewContext()
	cancel := ctx.GetCancelFunc()
	cancel()
	select {
	case <-ctx.Done():
		//Do nothing
	default:
		t.Error("Cancel didn't cancel!!")
	}
}

func TestGetID(t *testing.T) {
	ctx := NewContext()
	var cid uint64
	for cid = 0; cid < 100; cid = (cid + 1) * 2 {
		ctx.setClientId(cid)
		if ctx.GetClientId() != cid {
			t.Fatal("Client ID mismatch")
		}
	}
}

func TestGenericGetSet(t *testing.T) {
	ctx := NewContext()
	data := "ASDASDAS"
	key := "ASNSA"
	ctx.SetValue(key, data)
	if ctx.GetValue(key).(string) != data {
		t.Error("Get/Set differ")
	}
}
