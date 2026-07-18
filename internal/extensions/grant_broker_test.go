package extensions

import "testing"

func TestGrantBrokerRespondPending(t *testing.T) {
	b := NewGrantBroker()
	ch := make(chan bool, 1)
	b.pending["g1"] = ch
	if err := b.Respond("g1", true); err != nil {
		t.Fatal(err)
	}
	if !<-ch {
		t.Fatal("expected true")
	}
}
