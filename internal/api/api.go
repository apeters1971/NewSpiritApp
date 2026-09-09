package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
	"github.com/gorilla/websocket"
)

const (
	memberCookie     = "spirit_session"
	controllerCookie = "spirit_controller"
)

type Server struct {
	Store            *store.Store
	Hub              *hub.Hub
	ControllerSecret string
	ClientFS         fs.FS
	ControllerFS     fs.FS
	upgrader         websocket.Upgrader
}

func New(st *store.Store, h *hub.Hub, secret string, clientFS, controllerFS fs.FS) *Server {
	return &Server{
		Store:            st,
		Hub:              h,
		ControllerSecret: secret,
		ClientFS:         clientFS,
		ControllerFS:     controllerFS,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/catalog", s.handleCatalog)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("PATCH /api/me/info", s.handleMeInfo)
	mux.HandleFunc("POST /api/me/photo", s.handleMePhoto)
	mux.HandleFunc("DELETE /api/me/photo", s.handleDeleteMePhoto)
	mux.HandleFunc("GET /api/photos/{id}", s.handleGetPhoto)
	mux.HandleFunc("GET /api/dates", s.handleDates)
	mux.HandleFunc("POST /api/dates/{id}/vote", s.handleVote)
	mux.HandleFunc("POST /api/dates/{id}/poll", s.handlePollVote)
	mux.HandleFunc("POST /api/dates/{id}/comments", s.handleAddComment)
	mux.HandleFunc("GET /api/chats/{room}", s.handleChatList)
	mux.HandleFunc("POST /api/chats/{room}", s.handleChatPost)
	mux.HandleFunc("POST /api/chats/{room}/messages/{id}/react", s.handleChatReact)
	mux.HandleFunc("GET /ws/client", s.handleMemberWS)

	mux.HandleFunc("POST /api/controller/login", s.handleControllerLogin)
	mux.HandleFunc("POST /api/controller/logout", s.handleControllerLogout)
	mux.HandleFunc("GET /api/controller/state", s.handleControllerState)
	mux.HandleFunc("POST /api/controller/users", s.handleCreateUser)
	mux.HandleFunc("PATCH /api/controller/users/{id}", s.handleUpdateUser)
	mux.HandleFunc("PATCH /api/controller/users/{id}/info", s.handleControllerUserInfo)
	mux.HandleFunc("DELETE /api/controller/users/{id}", s.handleDeleteUser)
	mux.HandleFunc("POST /api/controller/users/{id}/photo", s.handleControllerPhoto)
	mux.HandleFunc("DELETE /api/controller/users/{id}/photo", s.handleDeleteControllerPhoto)
	mux.HandleFunc("POST /api/controller/dates", s.handleCreateDate)
	mux.HandleFunc("PATCH /api/controller/dates/{id}", s.handleUpdateDate)
	mux.HandleFunc("DELETE /api/controller/dates/{id}", s.handleDeleteDate)
	mux.HandleFunc("POST /api/controller/dates/{id}/status", s.handleDateStatus)
	mux.HandleFunc("POST /api/controller/dates/{id}/freeze", s.handleFreezePoll)
	mux.HandleFunc("GET /api/controller/chats/{room}", s.handleControllerChatList)
	mux.HandleFunc("POST /api/controller/chats/{room}", s.handleControllerChatPost)
	mux.HandleFunc("POST /api/controller/chats/{room}/messages/{id}/react", s.handleControllerChatReact)
	mux.HandleFunc("GET /ws/controller", s.handleControllerWS)

	mux.HandleFunc("GET /controller", s.serveControllerIndex)
	mux.HandleFunc("GET /controller/", s.serveControllerIndex)
	mux.Handle("GET /controller/static/", http.StripPrefix("/controller/static/", http.FileServer(http.FS(s.ControllerFS))))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(s.ClientFS))))
	mux.HandleFunc("GET /{$}", s.serveClientIndex)

	return withJSONHeaders(mux)
}

func withJSONHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func sessionCookie(name, value string, maxAge int, r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

func (s *Server) userFromRequest(r *http.Request) (store.User, error) {
	c, err := r.Cookie(memberCookie)
	if err != nil {
		return store.User{}, store.ErrUnauthorized
	}
	return s.Store.UserBySession(c.Value)
}

func (s *Server) requireController(w http.ResponseWriter, r *http.Request) bool {
	c, err := r.Cookie(controllerCookie)
	if err != nil || !s.Store.ValidControllerSession(c.Value) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, store.Catalog())
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, sid, err := s.Store.Login(body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	http.SetCookie(w, sessionCookie(memberCookie, sid, 60*60*24*30, r))
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(memberCookie); err == nil {
		s.Store.RevokeSession(c.Value)
	}
	http.SetCookie(w, sessionCookie(memberCookie, "", -1, r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func readUserInfo(r *http.Request) (string, string, string, error) {
	var body struct {
		Address  string `json:"address"`
		Phone    string `json:"phone"`
		Birthday string `json:"birthday"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", "", "", err
	}
	return body.Address, body.Phone, body.Birthday, nil
}

func (s *Server) handleMeInfo(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	address, phone, birthday, err := readUserInfo(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err = s.Store.SetUserInfo(user.ID, address, phone, birthday)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) canViewPhoto(r *http.Request) bool {
	if _, err := s.userFromRequest(r); err == nil {
		return true
	}
	c, err := r.Cookie(controllerCookie)
	return err == nil && s.Store.ValidControllerSession(c.Value)
}

func (s *Server) readNormalizedPhoto(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, store.PhotoMaxUpload)
	if err := r.ParseMultipartForm(store.PhotoMaxUpload); err != nil {
		return nil, fmt.Errorf("picture is too large")
	}
	f, _, err := r.FormFile("photo")
	if err != nil {
		return nil, fmt.Errorf("picture is required")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, store.PhotoMaxUpload+1))
	if err != nil {
		return nil, fmt.Errorf("picture is required")
	}
	if len(raw) > store.PhotoMaxUpload {
		return nil, fmt.Errorf("picture is too large")
	}
	return store.NormalizePhoto(raw)
}

func (s *Server) writeUserPhoto(w http.ResponseWriter, r *http.Request, userID string) {
	data, err := s.readNormalizedPhoto(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.SetPhoto(userID, data); err != nil {
		writeStoreError(w, err)
		return
	}
	user, err := s.Store.UserByID(userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleMePhoto(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	s.writeUserPhoto(w, r, user.ID)
}

func (s *Server) handleDeleteMePhoto(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.Store.DeletePhoto(user.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	user, err = s.Store.UserByID(user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleGetPhoto(w http.ResponseWriter, r *http.Request) {
	if !s.canViewPhoto(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	photo, err := s.Store.Photo(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.Header().Set("Content-Type", photo.MIME)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(photo.Data)
}

func (s *Server) handleControllerPhoto(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.writeUserPhoto(w, r, r.PathValue("id"))
}

func (s *Server) handleDeleteControllerPhoto(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	id := r.PathValue("id")
	if err := s.Store.DeletePhoto(id); err != nil {
		writeStoreError(w, err)
		return
	}
	user, err := s.Store.UserByID(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleDates(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	dates, err := s.Store.ListDateViews(&user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rank, err := s.Store.ChoirRanking(time.Now().Year())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	payload := map[string]any{
		"year":   rank.Year,
		"leader": rank.Leader,
	}
	for _, e := range rank.Entries {
		if e.UserID == user.ID {
			payload["myScore"] = e.Score
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dates":   dates,
		"ranking": payload,
	})
}

func (s *Server) handleVote(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Choice string `json:"choice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.Store.SetVote(user.ID, r.PathValue("id"), body.Choice); err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.Store.DateView(r.PathValue("id"), &user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"date": view})
}

func (s *Server) handlePollVote(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		OptionID string `json:"optionId"`
		Choice   string `json:"choice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.Store.SetPollVote(user.ID, r.PathValue("id"), body.OptionID, body.Choice); err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.Store.DateView(r.PathValue("id"), &user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"date": view})
}

func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := s.Store.AddComment(user.ID, r.PathValue("id"), body.Text); err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.Store.DateView(r.PathValue("id"), &user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"date": view})
}

func (s *Server) publishChat(room string, env hub.Envelope) {
	s.Hub.BroadcastToRoles(s.Store.ChatRoomRoles(room), env)
}

func (s *Server) handleChatList(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	msgs, err := s.Store.ListChatMessages(user.Role, r.PathValue("room"), user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) handleChatPost(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	msg, err := s.Store.AddChatMessage(user.ID, r.PathValue("room"), body.Text)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(msg.Room, hub.Envelope{Type: "chat", Data: msg})
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleChatReact(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	room := r.PathValue("room")
	msg, err := s.Store.ToggleChatReaction(user.ID, room, r.PathValue("id"), body.Emoji, false)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(room, hub.Envelope{Type: "react", Data: map[string]any{
		"room":      room,
		"messageId": msg.ID,
		"reactions": msg.Reactions,
	}})
	writeJSON(w, http.StatusOK, map[string]any{"message": msg})
}

func (s *Server) handleControllerChatList(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	msgs, err := s.Store.ListChatMessagesForRoom(r.PathValue("room"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) handleControllerChatPost(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	msg, err := s.Store.AddAdminChatMessage(r.PathValue("room"), body.Text)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(msg.Room, hub.Envelope{Type: "chat", Data: msg})
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleControllerChatReact(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	room := r.PathValue("room")
	msg, err := s.Store.ToggleChatReaction("", room, r.PathValue("id"), body.Emoji, true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(room, hub.Envelope{Type: "react", Data: map[string]any{
		"room":      room,
		"messageId": msg.ID,
		"reactions": msg.Reactions,
	}})
	writeJSON(w, http.StatusOK, map[string]any{"message": msg})
}

func (s *Server) handleControllerLogin(w http.ResponseWriter, r *http.Request) {
	if s.ControllerSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "CONTROLLER_SECRET not configured")
		return
	}
	var body struct {
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Secret != s.ControllerSecret {
		writeError(w, http.StatusUnauthorized, "invalid secret")
		return
	}
	sid, err := s.Store.CreateControllerSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, sessionCookie(controllerCookie, sid, 60*60*12, r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleControllerLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(controllerCookie); err == nil {
		s.Store.RevokeControllerSession(c.Value)
	}
	http.SetCookie(w, sessionCookie(controllerCookie, "", -1, r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleControllerState(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	users, err := s.Store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dates, err := s.Store.ListDateViews(nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rank, err := s.Store.ChoirRanking(time.Now().Year())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":   users,
		"dates":   dates,
		"online":  s.Hub.OnlineCount(),
		"ranking": rank,
	})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Subrole  string `json:"subrole"`
		Address  string `json:"address"`
		Phone    string `json:"phone"`
		Birthday string `json:"birthday"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err := s.Store.CreateUser(body.Nickname, body.Email, body.Password, body.Role, body.Subrole)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if body.Address != "" || body.Phone != "" || body.Birthday != "" {
		user, err = s.Store.SetUserInfo(user.ID, body.Address, body.Phone, body.Birthday)
		if err != nil {
			writeStoreError(w, err)
			return
		}
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Subrole  string `json:"subrole"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err := s.Store.UpdateUser(r.PathValue("id"), body.Nickname, body.Email, body.Password, body.Role, body.Subrole)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleControllerUserInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	address, phone, birthday, err := readUserInfo(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err := s.Store.SetUserInfo(r.PathValue("id"), address, phone, birthday)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	if err := s.Store.DeleteUser(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type pollOptionBody struct {
	ID       string `json:"id"`
	StartsAt string `json:"startsAt"`
	EndsAt   string `json:"endsAt"`
}

type dateBody struct {
	Title    string           `json:"title"`
	Category string           `json:"category"`
	StartsAt string           `json:"startsAt"`
	EndsAt   string           `json:"endsAt"`
	Location string           `json:"location"`
	Notes    string           `json:"notes"`
	Roles    []string         `json:"roles"`
	Bring    store.Bring      `json:"bring"`
	Options  []pollOptionBody `json:"options"`
}

func parsePollOptions(raw []pollOptionBody) ([]store.PollOptionInput, error) {
	out := make([]store.PollOptionInput, 0, len(raw))
	for _, item := range raw {
		if strings.TrimSpace(item.StartsAt) == "" {
			continue
		}
		starts, err := parseWhen(item.StartsAt)
		if err != nil {
			return nil, errors.New("invalid start time")
		}
		opt := store.PollOptionInput{ID: strings.TrimSpace(item.ID), StartsAt: starts}
		if strings.TrimSpace(item.EndsAt) != "" {
			t, err := parseWhen(item.EndsAt)
			if err != nil {
				return nil, errors.New("invalid end time")
			}
			opt.EndsAt = &t
		}
		out = append(out, opt)
	}
	return out, nil
}

func parseDateBody(r *http.Request) (dateBody, time.Time, *time.Time, []store.PollOptionInput, error) {
	var body dateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return body, time.Time{}, nil, nil, err
	}
	options, err := parsePollOptions(body.Options)
	if err != nil {
		return body, time.Time{}, nil, nil, err
	}
	var starts time.Time
	if strings.TrimSpace(body.StartsAt) != "" {
		starts, err = parseWhen(body.StartsAt)
		if err != nil {
			return body, time.Time{}, nil, nil, errors.New("invalid start time")
		}
	} else if len(options) < 2 {
		return body, time.Time{}, nil, nil, errors.New("invalid start time")
	}
	var ends *time.Time
	if strings.TrimSpace(body.EndsAt) != "" {
		t, err := parseWhen(body.EndsAt)
		if err != nil {
			return body, time.Time{}, nil, nil, errors.New("invalid end time")
		}
		ends = &t
	}
	return body, starts, ends, options, nil
}

func parseWhen(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("invalid time")
}

func (s *Server) handleCreateDate(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	body, starts, ends, options, err := parseDateBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	d, err := s.Store.CreateDate(body.Title, body.Category, starts, ends, body.Location, body.Notes, body.Roles, body.Bring, options)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.Store.DateView(d.ID, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"date": view})
}

func (s *Server) handleUpdateDate(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	body, starts, ends, options, err := parseDateBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.Store.UpdateDate(r.PathValue("id"), body.Title, body.Category, starts, ends, body.Location, body.Notes, body.Roles, body.Bring, options); err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.Store.DateView(r.PathValue("id"), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"date": view})
}

func (s *Server) handleDeleteDate(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	if err := s.Store.DeleteDate(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDateStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := s.Store.SetDateStatus(r.PathValue("id"), body.Status); err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.Store.DateView(r.PathValue("id"), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"date": view})
}

func (s *Server) handleFreezePoll(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		OptionID string `json:"optionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := s.Store.FreezePoll(r.PathValue("id"), body.OptionID); err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.Store.DateView(r.PathValue("id"), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"date": view})
}

func (s *Server) handleMemberWS(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := s.Hub.RegisterMember(conn, user.ID, user.Role)
	defer s.Hub.UnregisterMember(c)
	s.readLoop(conn)
}

func (s *Server) handleControllerWS(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(controllerCookie)
	if err != nil || !s.Store.ValidControllerSession(cookie.Value) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := s.Hub.RegisterController(conn)
	defer s.Hub.UnregisterController(c)
	s.readLoop(conn)
}

func (s *Server) readLoop(conn *websocket.Conn) {
	defer conn.Close()
	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) serveClientIndex(w http.ResponseWriter, r *http.Request) {
	serveFSFile(w, r, s.ClientFS, "index.html")
}

func (s *Server) serveControllerIndex(w http.ResponseWriter, r *http.Request) {
	serveFSFile(w, r, s.ControllerFS, "index.html")
}

func serveFSFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	f, err := fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, name, stat.ModTime(), rs)
		return
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, stat.ModTime(), bytes.NewReader(raw))
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
