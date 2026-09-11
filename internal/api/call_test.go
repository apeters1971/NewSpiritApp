package api

import (
	"testing"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
)

func TestCallInviteTracksPair(t *testing.T) {
	h := hub.New()
	h.RegisterMember(nil, "ada", "choir")
	h.RegisterMember(nil, "ben", "band")
	s := &Server{Hub: h, calls: map[string]string{}}
	ada := store.User{ID: "ada", Nickname: "Ada"}
	s.handleMemberCall(ada, "callInvite", "ada", "audio", nil, nil)
	if s.callPeer("ada") != "" {
		t.Fatal("self invite")
	}
	s.handleMemberCall(ada, "callInvite", "cara", "audio", nil, nil)
	if s.callPeer("ada") != "" {
		t.Fatal("offline invite should not stick")
	}
	s.handleMemberCall(ada, "callInvite", "ben", "video", nil, nil)
	if s.callPeer("ada") != "ben" || s.callPeer("ben") != "ada" {
		t.Fatalf("pair %q %q", s.callPeer("ada"), s.callPeer("ben"))
	}
	if s.clearCall("ada") != "ben" || s.callPeer("ben") != "" {
		t.Fatal("clear should drop both")
	}
}

func TestCallInviteBusyWhenPeerInCall(t *testing.T) {
	h := hub.New()
	h.RegisterMember(nil, "ada", "choir")
	h.RegisterMember(nil, "ben", "band")
	h.RegisterMember(nil, "cara", "choir")
	s := &Server{Hub: h, calls: map[string]string{}}
	s.setCall("ben", "cara")
	s.handleMemberCall(store.User{ID: "ada", Nickname: "Ada"}, "callInvite", "ben", "audio", nil, nil)
	if s.callPeer("ada") != "" {
		t.Fatal("busy peer should not start a second call")
	}
	if s.callPeer("ben") != "cara" {
		t.Fatal("existing call should stay")
	}
}
