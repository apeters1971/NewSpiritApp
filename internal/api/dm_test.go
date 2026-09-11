package api

import (
	"testing"

	"github.com/apeters/newspirit/internal/store"
)

func TestEphemeralDMClearsOnClose(t *testing.T) {
	s := &Server{dms: map[string][]store.ChatMessage{}}
	room := store.DMRoom("ada", "cara")
	s.appendDM(room, store.ChatMessage{ID: "1", Room: room, Text: "hi"})
	if got := s.listDM(room); len(got) != 1 || got[0].Text != "hi" {
		t.Fatalf("stored %+v", got)
	}
	s.clearDM(room)
	if got := s.listDM(room); len(got) != 0 {
		t.Fatalf("cleared %+v", got)
	}
}
