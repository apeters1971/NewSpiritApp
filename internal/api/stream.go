package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
	"github.com/gorilla/websocket"
)

func (s *Server) handleStreamStatus(w http.ResponseWriter, r *http.Request) {
	if _, err := s.userFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stream": s.Hub.LiveStream()})
}

func (s *Server) readMemberLoop(conn *websocket.Conn, user store.User) {
	defer conn.Close()
	conn.SetReadLimit(1 << 18)
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		s.handleMemberStream(user, raw)
	}
}

func (s *Server) handleMemberStream(user store.User, raw []byte) {
	var msg struct {
		Type string `json:"type"`
		Data struct {
			To        string          `json:"to"`
			SDP       json.RawMessage `json:"sdp"`
			Candidate json.RawMessage `json:"candidate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Type == "" {
		return
	}
	switch msg.Type {
	case "streamStart":
		fresh, err := s.Store.UserByID(user.ID)
		if err != nil || !fresh.Streamer {
			s.Hub.SendToUser(user.ID, hub.Envelope{Type: "streamDenied"})
			return
		}
		live, err := s.Hub.StartStream(fresh.ID, fresh.Nickname)
		if errors.Is(err, hub.ErrStreamBusy) {
			s.Hub.SendToUser(user.ID, hub.Envelope{Type: "streamBusy"})
			return
		}
		if err != nil {
			return
		}
		s.Hub.Broadcast(hub.Envelope{Type: "streamLive", Data: live})
	case "streamStop":
		if s.Hub.StopStream(user.ID) {
			s.Hub.Broadcast(hub.Envelope{Type: "streamEnded"})
		}
	case "streamWatch":
		live := s.Hub.LiveStream()
		if live == nil {
			s.Hub.SendToUser(user.ID, hub.Envelope{Type: "streamEnded"})
			return
		}
		s.Hub.SendToUser(live.UserID, hub.Envelope{
			Type: "streamWatch",
			Data: map[string]string{"from": user.ID, "nickname": user.Nickname},
		})
	case "streamOffer", "streamAnswer", "streamIce":
		to := msg.Data.To
		if to == "" || to == user.ID {
			return
		}
		live := s.Hub.LiveStream()
		if live == nil {
			return
		}
		if msg.Type == "streamOffer" && user.ID != live.UserID {
			return
		}
		if msg.Type == "streamAnswer" && to != live.UserID {
			return
		}
		if msg.Type == "streamIce" && user.ID != live.UserID && to != live.UserID {
			return
		}
		payload := map[string]any{"from": user.ID}
		if len(msg.Data.SDP) > 0 {
			payload["sdp"] = json.RawMessage(msg.Data.SDP)
		}
		if len(msg.Data.Candidate) > 0 {
			payload["candidate"] = json.RawMessage(msg.Data.Candidate)
		}
		s.Hub.SendToUser(to, hub.Envelope{Type: msg.Type, Data: payload})
	}
}
