package registry

import (
	"testing"
	"time"
)

func TestParseJSON(t *testing.T) {
	e, err := ParseJSON([]byte(`{"event":"up","source_id":"front-cam-01"}`))
	if err != nil {
		t.Fatal(err)
	}
	if e.ID() != "front-cam-01" || !e.IsUp() {
		t.Fatalf("%+v", e)
	}
	e2, err := ParseJSON([]byte(`{"event":"heartbeat","source":"cabin-1"}`))
	if err != nil || e2.ID() != "cabin-1" || !e2.IsUp() {
		t.Fatalf("%+v %v", e2, err)
	}
	e3, _ := ParseJSON([]byte(`{"event":"down","source_id":"x"}`))
	if !e3.IsDown() {
		t.Fatal(e3)
	}
}

func TestLiveDebounce(t *testing.T) {
	l := New(time.Minute)
	now := time.Now()
	if !l.Touch("a", now) {
		t.Fatal("first touch should emit")
	}
	if l.Touch("a", now.Add(10*time.Second)) {
		t.Fatal("within debounce should not emit")
	}
	if !l.Touch("a", now.Add(time.Minute+time.Second)) {
		t.Fatal("after debounce should emit")
	}
	l.Remove("a")
	if l.Count() != 0 {
		t.Fatal(l.Count())
	}
}
