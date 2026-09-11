package hub

import (
	"slices"
	"testing"
)

func TestOnlineUserIDsCountsPeopleOnce(t *testing.T) {
	h := New()
	if n := h.OnlineCount(); n != 0 || len(h.OnlineUserIDs()) != 0 {
		t.Fatalf("empty hub %d %v", n, h.OnlineUserIDs())
	}
	a1 := h.RegisterMember(nil, "ada", "choir")
	a2 := h.RegisterMember(nil, "ada", "choir")
	_ = h.RegisterMember(nil, "ben", "band")
	ids := h.OnlineUserIDs()
	slices.Sort(ids)
	if h.OnlineCount() != 2 || len(ids) != 2 || ids[0] != "ada" || ids[1] != "ben" {
		t.Fatalf("online %d %v", h.OnlineCount(), ids)
	}
	h.UnregisterMember(a1)
	if h.OnlineCount() != 2 {
		t.Fatalf("ada still online %d", h.OnlineCount())
	}
	if !h.IsOnline("ada") || !h.IsOnline("ben") || h.IsOnline("cara") {
		t.Fatal("online check")
	}
	h.UnregisterMember(a2)
	ids = h.OnlineUserIDs()
	if h.OnlineCount() != 1 || len(ids) != 1 || ids[0] != "ben" {
		t.Fatalf("after ada left %d %v", h.OnlineCount(), ids)
	}
}

func TestLiveStreamOneAtATime(t *testing.T) {
	h := New()
	live, err := h.StartStream("ada", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if live.UserID != "ada" || h.LiveStream() == nil {
		t.Fatalf("live %+v", live)
	}
	if _, err := h.StartStream("ben", "Ben"); err != ErrStreamBusy {
		t.Fatalf("busy %v", err)
	}
	if _, err := h.StartStream("ada", "Ada"); err != nil {
		t.Fatal(err)
	}
	if h.StopStream("ben") {
		t.Fatal("other user stopped stream")
	}
	if !h.StopStream("ada") || h.LiveStream() != nil {
		t.Fatal("stream should end")
	}
}
