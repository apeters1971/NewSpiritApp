package hub

import "testing"

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
