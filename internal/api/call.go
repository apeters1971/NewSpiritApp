package api

import (
	"encoding/json"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
)

func callKind(kind string) string {
	if kind == "video" {
		return "video"
	}
	return "audio"
}

func isCallSignal(typ string) bool {
	switch typ {
	case "callInvite", "callAnswer", "callIce", "callDecline", "callHangup", "callBusy", "callCancel":
		return true
	default:
		return false
	}
}

func (s *Server) setCall(a, b string) {
	if a == "" || b == "" || a == b {
		return
	}
	s.callMu.Lock()
	defer s.callMu.Unlock()
	s.calls[a] = b
	s.calls[b] = a
}

func (s *Server) callPeer(id string) string {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	return s.calls[id]
}

func (s *Server) clearCall(id string) string {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	peer := s.calls[id]
	delete(s.calls, id)
	delete(s.calls, peer)
	return peer
}

func (s *Server) handleMemberCall(user store.User, typ, to, kind string, sdp, cand json.RawMessage) {
	if !isCallSignal(typ) || to == "" || to == user.ID {
		return
	}
	if typ == "callInvite" {
		if !s.Hub.IsOnline(to) {
			s.Hub.SendToUser(user.ID, hub.Envelope{Type: "callBusy", Data: map[string]any{"from": to}})
			return
		}
		if other := s.callPeer(to); other != "" && other != user.ID {
			s.Hub.SendToUser(user.ID, hub.Envelope{Type: "callBusy", Data: map[string]any{"from": to}})
			return
		}
		s.setCall(user.ID, to)
	}
	if typ == "callHangup" || typ == "callDecline" || typ == "callBusy" || typ == "callCancel" {
		s.clearCall(user.ID)
	}
	payload := map[string]any{"from": user.ID, "nickname": user.Nickname, "kind": callKind(kind)}
	if len(sdp) > 0 {
		payload["sdp"] = json.RawMessage(sdp)
	}
	if len(cand) > 0 {
		payload["candidate"] = json.RawMessage(cand)
	}
	s.Hub.SendToUser(to, hub.Envelope{Type: typ, Data: payload})
}
