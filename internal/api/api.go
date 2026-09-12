package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
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
	Store               *store.Store
	Hub                 *hub.Hub
	ControllerSecret    string
	CloudflareToken     string
	CloudflareAccountID string
	ClientFS            fs.FS
	ControllerFS        fs.FS
	upgrader            websocket.Upgrader
	dmMu                sync.Mutex
	dms                 map[string][]store.ChatMessage
	callMu              sync.Mutex
	calls               map[string]string
}

type Options struct {
	ControllerSecret    string
	CloudflareToken     string
	CloudflareAccountID string
}

func New(st *store.Store, h *hub.Hub, opt Options, clientFS, controllerFS fs.FS) *Server {
	return &Server{
		Store:               st,
		Hub:                 h,
		ControllerSecret:    opt.ControllerSecret,
		CloudflareToken:     strings.TrimSpace(opt.CloudflareToken),
		CloudflareAccountID: strings.TrimSpace(opt.CloudflareAccountID),
		ClientFS:            clientFS,
		ControllerFS:        controllerFS,
		dms:                 map[string][]store.ChatMessage{},
		calls:               map[string]string{},
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
	mux.HandleFunc("PATCH /api/me/password", s.handleMePassword)
	mux.HandleFunc("PATCH /api/me/info", s.handleMeInfo)
	mux.HandleFunc("PATCH /api/me/channels/{n}", s.handleMeChannel)
	mux.HandleFunc("GET /api/channels", s.handleMemberChannels)
	mux.HandleFunc("PATCH /api/channels/{n}", s.handleMemberAssignChannel)
	mux.HandleFunc("POST /api/me/photo", s.handleMePhoto)
	mux.HandleFunc("DELETE /api/me/photo", s.handleDeleteMePhoto)
	mux.HandleFunc("GET /api/photos/{id}", s.handleGetPhoto)
	mux.HandleFunc("GET /api/dates", s.handleDates)
	mux.HandleFunc("POST /api/dates/{id}/vote", s.handleVote)
	mux.HandleFunc("POST /api/dates/{id}/poll", s.handlePollVote)
	mux.HandleFunc("POST /api/dates/{id}/comments", s.handleAddComment)
	mux.HandleFunc("GET /api/dates/{id}/gallery", s.handleGalleryList)
	mux.HandleFunc("POST /api/dates/{id}/gallery", s.handleGalleryUpload)
	mux.HandleFunc("GET /api/dates/{id}/gallery/{fileId}", s.handleGalleryFile)
	mux.HandleFunc("GET /api/galleries", s.handleGalleryHub)
	mux.HandleFunc("GET /api/galleries/{album}", s.handleAlbumList)
	mux.HandleFunc("POST /api/galleries/{album}", s.handleAlbumUpload)
	mux.HandleFunc("GET /api/galleries/{album}/files/{fileId}", s.handleAlbumFile)
	mux.HandleFunc("GET /api/dates/{id}/promo", s.handlePromoList)
	mux.HandleFunc("GET /api/dates/{id}/promo/{fileId}", s.handlePromoFile)
	mux.HandleFunc("POST /api/dms/{id}", s.handleDMOpen)
	mux.HandleFunc("POST /api/dms/{id}/messages", s.handleDMPost)
	mux.HandleFunc("POST /api/dms/{id}/close", s.handleDMClose)
	mux.HandleFunc("GET /api/chats/{room}", s.handleChatList)
	mux.HandleFunc("POST /api/chats/{room}", s.handleChatPost)
	mux.HandleFunc("POST /api/chats/{room}/voice", s.handleChatVoice)
	mux.HandleFunc("GET /api/chats/{room}/messages/{id}/voice", s.handleChatVoiceFile)
	mux.HandleFunc("POST /api/chats/{room}/media", s.handleChatMedia)
	mux.HandleFunc("GET /api/chats/{room}/messages/{id}/media", s.handleChatMediaFile)
	mux.HandleFunc("POST /api/chats/{room}/read", s.handleChatRead)
	mux.HandleFunc("POST /api/chats/{room}/messages/{id}/react", s.handleChatReact)
	mux.HandleFunc("PATCH /api/chats/{room}/messages/{id}", s.handleChatEdit)
	mux.HandleFunc("DELETE /api/chats/{room}/messages/{id}", s.handleChatDelete)
	mux.HandleFunc("GET /api/archive", s.handleArchiveList)
	mux.HandleFunc("POST /api/archive", s.handleArchiveCreate)
	mux.HandleFunc("GET /api/archive/{id}", s.handleArchiveItem)
	mux.HandleFunc("GET /api/archive/{id}/files/{fileId}", s.handleArchiveFile)
	mux.HandleFunc("POST /api/archive/{id}/files", s.handleArchiveUpload)
	mux.HandleFunc("POST /api/archive/{id}/links", s.handleArchiveAddLink)
	mux.HandleFunc("GET /api/proposals", s.handleProposals)
	mux.HandleFunc("POST /api/proposals", s.handleCreateProposal)
	mux.HandleFunc("GET /api/directory", s.handleDirectory)
	mux.HandleFunc("GET /api/me/calendar", s.handleMeCalendar)
	mux.HandleFunc("GET /api/stream", s.handleStreamStatus)
	mux.HandleFunc("GET /calendar/{token}", s.handleCalendarFeed)
	mux.HandleFunc("GET /ws/client", s.handleMemberWS)

	mux.HandleFunc("POST /api/controller/login", s.handleControllerLogin)
	mux.HandleFunc("POST /api/controller/logout", s.handleControllerLogout)
	mux.HandleFunc("GET /api/controller/state", s.handleControllerState)
	mux.HandleFunc("PATCH /api/controller/settings", s.handleControllerSettings)
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
	mux.HandleFunc("POST /api/controller/dates/{id}/attendance", s.handleControllerAttendance)
	mux.HandleFunc("POST /api/controller/dates/{id}/freeze", s.handleFreezePoll)
	mux.HandleFunc("GET /api/controller/dates/{id}/gallery", s.handleControllerGalleryList)
	mux.HandleFunc("POST /api/controller/dates/{id}/gallery", s.handleControllerGalleryUpload)
	mux.HandleFunc("GET /api/controller/dates/{id}/gallery/{fileId}", s.handleControllerGalleryFile)
	mux.HandleFunc("DELETE /api/controller/dates/{id}/gallery/{fileId}", s.handleControllerGalleryDelete)
	mux.HandleFunc("GET /api/controller/dates/{id}/promo", s.handleControllerPromoList)
	mux.HandleFunc("POST /api/controller/dates/{id}/promo", s.handleControllerPromoAdd)
	mux.HandleFunc("PATCH /api/controller/dates/{id}/promo", s.handleControllerPromoNote)
	mux.HandleFunc("GET /api/controller/dates/{id}/promo/{fileId}", s.handleControllerPromoFile)
	mux.HandleFunc("DELETE /api/controller/dates/{id}/promo/{fileId}", s.handleControllerPromoDelete)
	mux.HandleFunc("GET /api/controller/chats/{room}", s.handleControllerChatList)
	mux.HandleFunc("POST /api/controller/chats/{room}", s.handleControllerChatPost)
	mux.HandleFunc("POST /api/controller/chats/{room}/voice", s.handleControllerChatVoice)
	mux.HandleFunc("GET /api/controller/chats/{room}/messages/{id}/voice", s.handleControllerChatVoiceFile)
	mux.HandleFunc("POST /api/controller/chats/{room}/media", s.handleControllerChatMedia)
	mux.HandleFunc("GET /api/controller/chats/{room}/messages/{id}/media", s.handleControllerChatMediaFile)
	mux.HandleFunc("POST /api/controller/chats/{room}/read", s.handleControllerChatRead)
	mux.HandleFunc("POST /api/controller/chats/{room}/messages/{id}/react", s.handleControllerChatReact)
	mux.HandleFunc("PATCH /api/controller/chats/{room}/messages/{id}", s.handleControllerChatEdit)
	mux.HandleFunc("DELETE /api/controller/chats/{room}/messages/{id}", s.handleControllerChatDelete)
	mux.HandleFunc("GET /api/controller/archive", s.handleControllerArchiveList)
	mux.HandleFunc("POST /api/controller/archive", s.handleControllerArchiveCreate)
	mux.HandleFunc("PATCH /api/controller/archive/{id}", s.handleControllerArchiveUpdate)
	mux.HandleFunc("DELETE /api/controller/archive/{id}", s.handleControllerArchiveDelete)
	mux.HandleFunc("POST /api/controller/archive/{id}/files", s.handleControllerArchiveUpload)
	mux.HandleFunc("POST /api/controller/archive/{id}/links", s.handleControllerArchiveAddLink)
	mux.HandleFunc("PATCH /api/controller/archive/{id}/files/{fileId}", s.handleControllerArchiveUpdateFile)
	mux.HandleFunc("DELETE /api/controller/archive/{id}/files/{fileId}", s.handleControllerArchiveDeleteFile)
	mux.HandleFunc("GET /api/controller/archive/{id}/files/{fileId}", s.handleControllerArchiveFile)
	mux.HandleFunc("POST /api/controller/archive/{id}/accept", s.handleControllerArchiveAccept)
	mux.HandleFunc("POST /api/controller/archive/{id}/files/{fileId}/accept", s.handleControllerArchiveAcceptFile)
	mux.HandleFunc("PATCH /api/controller/channels/{n}", s.handleControllerChannel)
	mux.HandleFunc("PATCH /api/controller/proposals/{id}", s.handleControllerUpdateProposal)
	mux.HandleFunc("DELETE /api/controller/proposals/{id}", s.handleControllerDeleteProposal)
	mux.HandleFunc("GET /ws/controller", s.handleControllerWS)

	mux.HandleFunc("GET /manifest.webmanifest", s.serveManifest)
	mux.HandleFunc("GET /sw.js", s.serveServiceWorker)
	mux.HandleFunc("GET /controller", s.serveControllerIndex)
	mux.HandleFunc("GET /controller/", s.serveControllerIndex)
	mux.Handle("GET /controller/static/", http.StripPrefix("/controller/static/", http.FileServer(http.FS(s.ControllerFS))))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(s.ClientFS))))
	mux.HandleFunc("GET /{$}", s.serveClientIndex)

	return s.withPasswordGate(withJSONHeaders(mux))
}

func (s *Server) withPasswordGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.blocksUntilPasswordChange(r) {
			user, err := s.userFromRequest(r)
			if err == nil && user.MustChangePassword {
				writeError(w, http.StatusForbidden, "password change required")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) blocksUntilPasswordChange(r *http.Request) bool {
	p := r.URL.Path
	switch {
	case strings.HasPrefix(p, "/api/controller"), p == "/api/login", p == "/api/logout", p == "/api/catalog", p == "/api/me", p == "/api/me/password":
		return false
	case strings.HasPrefix(p, "/api/"), strings.HasPrefix(p, "/ws/"):
		return true
	default:
		return false
	}
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
	_ = s.Store.TouchLastConnected(user.ID)
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
	unread, err := s.Store.MemberChatUnread(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":       user,
		"unread":     unread,
		"stream":     s.Hub.LiveStream(),
		"newsTicker": s.Store.NewsTicker(),
		"online":     s.onlinePeople(),
	})
}

func (s *Server) handleMePassword(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err = s.Store.ChangeOwnPassword(user.ID, body.Password)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func readUserInfo(r *http.Request) (address, phone, birthday, altEmail string, err error) {
	var body struct {
		Address  string `json:"address"`
		Phone    string `json:"phone"`
		Birthday string `json:"birthday"`
		AltEmail string `json:"altEmail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", "", "", "", err
	}
	return body.Address, body.Phone, body.Birthday, body.AltEmail, nil
}

func (s *Server) handleMeInfo(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	address, phone, birthday, altEmail, err := readUserInfo(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err = s.Store.SetUserInfo(user.ID, address, phone, birthday, altEmail)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleMeChannel(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var n int
	if _, err := fmt.Sscanf(r.PathValue("n"), "%d", &n); err != nil {
		writeError(w, http.StatusBadRequest, "unknown channel")
		return
	}
	var body struct {
		Comment string `json:"comment"`
		V48     bool   `json:"v48"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if _, err := s.Store.SetChannelComment(user.ID, n, body.Comment, body.V48); err != nil {
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

func (s *Server) requireChannelAssigner(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return store.User{}, false
	}
	if !store.CanAssignChannels(user.Role) {
		writeError(w, http.StatusForbidden, "this is not for your role")
		return user, false
	}
	return user, true
}

func (s *Server) handleMemberChannels(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireChannelAssigner(w, r); !ok {
		return
	}
	channels, err := s.Store.ListChannels()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	people, err := s.Store.ChannelPeople()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": channels, "people": people})
}

func (s *Server) handleMemberAssignChannel(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireChannelAssigner(w, r); !ok {
		return
	}
	var n int
	if _, err := fmt.Sscanf(r.PathValue("n"), "%d", &n); err != nil {
		writeError(w, http.StatusBadRequest, "unknown channel")
		return
	}
	var body struct {
		UserID  string `json:"userId"`
		Comment string `json:"comment"`
		V48     bool   `json:"v48"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ch, err := s.Store.SetChannel(n, body.UserID, body.Comment, body.V48)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"channel": ch})
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
		"year":          rank.Year,
		"leaders":       rank.Leaders,
		"events":        rank.Events,
		"participation": rank.Participation,
	}
	for _, e := range rank.Entries {
		if e.UserID == user.ID {
			payload["myYes"] = e.Yes
			break
		}
	}
	unread, err := s.Store.MemberChatUnread(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dates":   dates,
		"ranking": payload,
		"unread":  unread,
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

func (s *Server) dmPeer(r *http.Request, user store.User) (store.User, string, error) {
	peer, err := s.Store.UserByID(r.PathValue("id"))
	if err != nil {
		return store.User{}, "", err
	}
	if peer.ID == user.ID {
		return store.User{}, "", fmt.Errorf("cannot chat with yourself")
	}
	room := store.DMRoom(user.ID, peer.ID)
	if room == "" {
		return store.User{}, "", fmt.Errorf("cannot chat with yourself")
	}
	return peer, room, nil
}

func (s *Server) listDM(room string) []store.ChatMessage {
	s.dmMu.Lock()
	defer s.dmMu.Unlock()
	src := s.dms[room]
	out := make([]store.ChatMessage, len(src))
	copy(out, src)
	return out
}

func (s *Server) appendDM(room string, msg store.ChatMessage) {
	s.dmMu.Lock()
	defer s.dmMu.Unlock()
	s.dms[room] = append(s.dms[room], msg)
}

func (s *Server) clearDM(room string) {
	s.dmMu.Lock()
	defer s.dmMu.Unlock()
	delete(s.dms, room)
}

func (s *Server) publishDM(room string, env hub.Envelope) {
	left, right, ok := store.ParseDMRoom(room)
	if !ok {
		return
	}
	s.Hub.SendToUser(left, env)
	s.Hub.SendToUser(right, env)
}

func (s *Server) handleDMOpen(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	peer, room, err := s.dmPeer(r, user)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !s.Hub.IsOnline(peer.ID) {
		writeError(w, http.StatusBadRequest, "this person is not online")
		return
	}
	msgs := s.listDM(room)
	s.Hub.SendToUser(peer.ID, hub.Envelope{
		Type: "dmOpen",
		Data: map[string]any{
			"room":     room,
			"peer":     store.OnlinePerson{ID: user.ID, Nickname: user.Nickname},
			"messages": msgs,
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"room":     room,
		"peer":     store.OnlinePerson{ID: peer.ID, Nickname: peer.Nickname},
		"messages": msgs,
	})
}

func (s *Server) handleDMPost(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_, room, err := s.dmPeer(r, user)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	msg, err := store.NewEphemeralChatMessage(user, room, body.Text)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.appendDM(room, msg)
	s.publishDM(room, hub.Envelope{Type: "chat", Data: msg})
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleDMClose(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_, room, err := s.dmPeer(r, user)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.clearDM(room)
	s.publishDM(room, hub.Envelope{Type: "dmClose", Data: map[string]any{"room": room}})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChatList(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	room := r.PathValue("room")
	msgs, err := s.Store.ListChatMessages(user.Role, room, user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	lastSeen := s.Store.ChatLastSeen(user.ID, room)
	_ = s.Store.MarkChatRead(user.ID, room, user.Role)
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs, "lastSeen": lastSeen})
}

func (s *Server) handleChatRead(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.Store.MarkChatRead(user.ID, r.PathValue("room"), user.Role); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
	_ = s.Store.MarkChatRead(user.ID, msg.Room, user.Role)
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleChatVoice(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	name, f, durationMs, err := s.readChatVoiceUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()
	msg, err := s.Store.AddChatVoice(user.ID, r.PathValue("room"), name, f, durationMs)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(msg.Room, hub.Envelope{Type: "chat", Data: msg})
	s.queueVoiceTranscript(msg)
	_ = s.Store.MarkChatRead(user.ID, msg.Room, user.Role)
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleChatVoiceFile(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	msg, path, err := s.Store.ChatVoiceFile(user.Role, r.PathValue("room"), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeChatVoiceFile(w, r, msg, path)
}

func (s *Server) handleChatMedia(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	name, f, err := s.readChatMediaUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()
	msg, err := s.Store.AddChatMedia(user.ID, r.PathValue("room"), name, f)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(msg.Room, hub.Envelope{Type: "chat", Data: msg})
	_ = s.Store.MarkChatRead(user.ID, msg.Room, user.Role)
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleChatMediaFile(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	msg, path, err := s.Store.ChatMediaFile(user.Role, r.PathValue("room"), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeChatMediaFile(w, r, msg, path)
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

func (s *Server) handleChatEdit(w http.ResponseWriter, r *http.Request) {
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
	room := r.PathValue("room")
	msg, err := s.Store.UpdateChatMessage(user.ID, room, r.PathValue("id"), body.Text, false)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(room, hub.Envelope{Type: "chatUpdate", Data: msg})
	writeJSON(w, http.StatusOK, map[string]any{"message": msg})
}

func (s *Server) handleChatDelete(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	room := r.PathValue("room")
	id := r.PathValue("id")
	if err := s.Store.DeleteChatMessage(user.ID, room, id, false); err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(room, hub.Envelope{Type: "chatDelete", Data: map[string]any{
		"room":      room,
		"messageId": id,
	}})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleControllerChatList(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	room := r.PathValue("room")
	msgs, err := s.Store.ListChatMessagesForRoom(room)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	lastSeen := s.Store.ChatLastSeen(store.ChatReadController, room)
	_ = s.Store.MarkChatRead(store.ChatReadController, room, "")
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs, "lastSeen": lastSeen})
}

func (s *Server) handleControllerChatRead(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	if err := s.Store.MarkChatRead(store.ChatReadController, r.PathValue("room"), ""); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
	_ = s.Store.MarkChatRead(store.ChatReadController, msg.Room, "")
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleControllerChatVoice(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	name, f, durationMs, err := s.readChatVoiceUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()
	msg, err := s.Store.AddAdminChatVoice(r.PathValue("room"), name, f, durationMs)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(msg.Room, hub.Envelope{Type: "chat", Data: msg})
	s.queueVoiceTranscript(msg)
	_ = s.Store.MarkChatRead(store.ChatReadController, msg.Room, "")
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleControllerChatVoiceFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	msg, path, err := s.Store.ChatVoiceFileForRoom(r.PathValue("room"), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeChatVoiceFile(w, r, msg, path)
}

func (s *Server) handleControllerChatMedia(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	name, f, err := s.readChatMediaUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()
	msg, err := s.Store.AddAdminChatMedia(r.PathValue("room"), name, f)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(msg.Room, hub.Envelope{Type: "chat", Data: msg})
	_ = s.Store.MarkChatRead(store.ChatReadController, msg.Room, "")
	writeJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

func (s *Server) handleControllerChatMediaFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	msg, path, err := s.Store.ChatMediaFileForRoom(r.PathValue("room"), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeChatMediaFile(w, r, msg, path)
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

func (s *Server) handleControllerChatEdit(w http.ResponseWriter, r *http.Request) {
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
	room := r.PathValue("room")
	msg, err := s.Store.UpdateChatMessage("", room, r.PathValue("id"), body.Text, true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(room, hub.Envelope{Type: "chatUpdate", Data: msg})
	writeJSON(w, http.StatusOK, map[string]any{"message": msg})
}

func (s *Server) handleControllerChatDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	room := r.PathValue("room")
	id := r.PathValue("id")
	if err := s.Store.DeleteChatMessage("", room, id, true); err != nil {
		writeStoreError(w, err)
		return
	}
	s.publishChat(room, hub.Envelope{Type: "chatDelete", Data: map[string]any{
		"room":      room,
		"messageId": id,
	}})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) readChatVoiceUpload(w http.ResponseWriter, r *http.Request) (string, io.ReadCloser, int, error) {
	max := int64(store.ChatVoiceMax) + 512<<10
	r.Body = http.MaxBytesReader(w, r.Body, max)
	if err := r.ParseMultipartForm(store.ChatVoiceMax); err != nil {
		return "", nil, 0, fmt.Errorf("file is too large")
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return "", nil, 0, fmt.Errorf("file is required")
	}
	name := ""
	if hdr != nil {
		name = hdr.Filename
	}
	durationMs, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("durationMs")))
	return name, f, durationMs, nil
}

func writeChatVoiceFile(w http.ResponseWriter, r *http.Request, msg store.ChatMessage, path string) {
	mime := msg.MIME
	if mime == "" {
		mime = "audio/webm"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Disposition", `inline; filename="voice"`)
	http.ServeFile(w, r, path)
}

func (s *Server) readChatMediaUpload(w http.ResponseWriter, r *http.Request) (string, io.ReadCloser, error) {
	max := int64(store.ChatVideoMax) + 512<<10
	r.Body = http.MaxBytesReader(w, r.Body, max)
	if err := r.ParseMultipartForm(store.ChatVideoMax); err != nil {
		return "", nil, fmt.Errorf("file is too large")
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return "", nil, fmt.Errorf("file is required")
	}
	name := ""
	if hdr != nil {
		name = hdr.Filename
	}
	return name, f, nil
}

func writeChatMediaFile(w http.ResponseWriter, r *http.Request, msg store.ChatMessage, path string) {
	mime := msg.MIME
	if mime == "" {
		if msg.Kind == store.ChatKindVideo {
			mime = "video/mp4"
		} else {
			mime = "image/jpeg"
		}
	}
	name := "image"
	if msg.Kind == store.ChatKindVideo {
		name = "video"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	http.ServeFile(w, r, path)
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
	archive, err := s.Store.ListArchive("")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	channels, err := s.Store.ListChannels()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	proposals, err := s.Store.ListProposals()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	unread, err := s.Store.ControllerChatUnread()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":      users,
		"dates":      dates,
		"online":     s.Hub.OnlineCount(),
		"ranking":    rank,
		"adminAlias": s.Store.AdminAlias(),
		"newsTicker": s.Store.NewsTicker(),
		"archive":    archive,
		"channels":   channels,
		"proposals":  proposals,
		"unread":     unread,
	})
}

func (s *Server) handleControllerSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		AdminAlias string `json:"adminAlias"`
		NewsTicker string `json:"newsTicker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	alias, err := s.Store.SetAdminAlias(body.AdminAlias)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	ticker, err := s.Store.SetNewsTicker(body.NewsTicker)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"adminAlias": alias, "newsTicker": ticker})
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
		AltEmail string `json:"altEmail"`
		Streamer bool   `json:"streamer"`
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
	if body.Address != "" || body.Phone != "" || body.Birthday != "" || body.AltEmail != "" {
		user, err = s.Store.SetUserInfo(user.ID, body.Address, body.Phone, body.Birthday, body.AltEmail)
		if err != nil {
			writeStoreError(w, err)
			return
		}
	}
	if body.Streamer {
		user, err = s.Store.SetUserStreamer(user.ID, true)
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
		Streamer bool   `json:"streamer"`
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
	user, err = s.Store.SetUserStreamer(user.ID, body.Streamer)
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
	address, phone, birthday, altEmail, err := readUserInfo(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err := s.Store.SetUserInfo(r.PathValue("id"), address, phone, birthday, altEmail)
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
	Schedule string           `json:"schedule"`
	Roles    []string         `json:"roles"`
	Bring    store.Bring      `json:"bring"`
	Options  []pollOptionBody `json:"options"`
	TitleIDs []string         `json:"titleIds"`
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
	d, err := s.Store.CreateDate(body.Title, body.Category, starts, ends, body.Location, body.Notes, body.Schedule, body.Roles, body.Bring, options)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := s.Store.SetDateTitles(d.ID, body.TitleIDs); err != nil {
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
	if _, err := s.Store.UpdateDate(r.PathValue("id"), body.Title, body.Category, starts, ends, body.Location, body.Notes, body.Schedule, body.Roles, body.Bring, options); err != nil {
		writeStoreError(w, err)
		return
	}
	if err := s.Store.SetDateTitles(r.PathValue("id"), body.TitleIDs); err != nil {
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

func (s *Server) handleControllerAttendance(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		UserID     string `json:"userId"`
		Attendance string `json:"attendance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.Store.SetDateAttendance(r.PathValue("id"), body.UserID, body.Attendance); err != nil {
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

func writeArchiveFile(w http.ResponseWriter, r *http.Request, f store.ArchiveFile) {
	if f.Kind == store.ArchiveKindLink {
		target := strings.TrimSpace(f.URL)
		if target == "" {
			target = strings.TrimSpace(string(f.Data))
		}
		if target == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", f.MIME)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	name := f.Name
	if name == "" {
		name = "file"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, name))
	http.ServeContent(w, r, name, f.UpdatedAt, bytes.NewReader(f.Data))
}

func (s *Server) handleArchiveCreate(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Title    string `json:"title"`
		Composer string `json:"composer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := s.Store.CreateMemberArchiveItem(body.Title, body.Composer, user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) applyArchiveUpload(w http.ResponseWriter, r *http.Request, id, createdBy string, pending bool) {
	name, data, err := s.readArchiveUpload(w, r, store.ArchiveKindAudio)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		kind = strings.TrimSpace(r.URL.Query().Get("kind"))
	}
	if label := strings.TrimSpace(r.FormValue("name")); label != "" {
		name = label
	}
	role := strings.TrimSpace(r.FormValue("role"))
	var item store.ArchiveItem
	if pending {
		item, err = s.Store.AddMemberArchiveFile(id, kind, role, name, createdBy, data)
	} else {
		item, err = s.Store.AddArchiveFile(id, kind, role, name, data)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) applyArchiveLink(w http.ResponseWriter, r *http.Request, id, createdBy string, pending bool) {
	var body struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var item store.ArchiveItem
	var err error
	if pending {
		item, err = s.Store.AddMemberArchiveLink(id, body.Name, body.URL, createdBy)
	} else {
		item, err = s.Store.AddArchiveLink(id, body.Name, body.URL)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleArchiveAddLink(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := r.PathValue("id")
	if err := s.Store.MemberCanAccessArchive(user.ID, id); err != nil {
		writeStoreError(w, err)
		return
	}
	s.applyArchiveLink(w, r, id, user.ID, true)
}

func (s *Server) handleArchiveUpload(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := r.PathValue("id")
	if err := s.Store.MemberCanAccessArchive(user.ID, id); err != nil {
		writeStoreError(w, err)
		return
	}
	s.applyArchiveUpload(w, r, id, user.ID, true)
}

func (s *Server) handleArchiveList(w http.ResponseWriter, r *http.Request) {
	if _, err := s.userFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	items, err := s.Store.ListArchive(r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"archive": items})
}

func (s *Server) handleArchiveItem(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := r.PathValue("id")
	if err := s.Store.MemberCanAccessArchive(user.ID, id); err != nil {
		writeStoreError(w, err)
		return
	}
	item, err := s.Store.ArchiveItem(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleArchiveFile(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if err := s.Store.MemberCanDownloadArchiveFile(user.ID, id, fileID); err != nil {
		writeStoreError(w, err)
		return
	}
	f, err := s.Store.GetArchiveFile(id, fileID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeArchiveFile(w, r, f)
}

func (s *Server) handleControllerArchiveList(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	items, err := s.Store.ListArchive(r.URL.Query().Get("q"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleControllerArchiveCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Title    string `json:"title"`
		Composer string `json:"composer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := s.Store.CreateArchiveItem(body.Title, body.Composer)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleControllerArchiveUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Title    string `json:"title"`
		Composer string `json:"composer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := s.Store.UpdateArchiveItem(r.PathValue("id"), body.Title, body.Composer)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleControllerArchiveDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	if err := s.Store.DeleteArchiveItem(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) readArchiveUpload(w http.ResponseWriter, r *http.Request, kind string) (string, []byte, error) {
	max := int64(store.ArchiveMaxBytes(kind)) + 512<<10
	r.Body = http.MaxBytesReader(w, r.Body, max)
	if err := r.ParseMultipartForm(max); err != nil {
		return "", nil, fmt.Errorf("file is too large")
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return "", nil, fmt.Errorf("file is required")
	}
	defer f.Close()
	limit := int64(store.ArchiveMaxBytes(kind)) + 1
	raw, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return "", nil, fmt.Errorf("file is required")
	}
	if int64(len(raw)) >= limit {
		return "", nil, fmt.Errorf("file is too large")
	}
	name := ""
	if hdr != nil {
		name = hdr.Filename
	}
	return name, raw, nil
}

func (s *Server) handleControllerArchiveUpload(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.applyArchiveUpload(w, r, r.PathValue("id"), "", false)
}

func (s *Server) handleControllerArchiveAddLink(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.applyArchiveLink(w, r, r.PathValue("id"), "", false)
}

func (s *Server) handleControllerArchiveUpdateFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Name string `json:"name"`
		Role string `json:"role"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	id := r.PathValue("id")
	fileID := r.PathValue("fileId")
	var item store.ArchiveItem
	var err error
	if strings.TrimSpace(body.URL) != "" {
		item, err = s.Store.UpdateArchiveLink(id, fileID, body.Name, body.URL)
	} else {
		item, err = s.Store.UpdateArchiveFile(id, fileID, body.Name, body.Role)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleControllerArchiveDeleteFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	item, err := s.Store.DeleteArchiveFile(r.PathValue("id"), r.PathValue("fileId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleControllerArchiveFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	f, err := s.Store.GetArchiveFile(r.PathValue("id"), r.PathValue("fileId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeArchiveFile(w, r, f)
}

func (s *Server) handleControllerArchiveAccept(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	item, err := s.Store.AcceptArchiveItem(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleControllerArchiveAcceptFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	item, err := s.Store.AcceptArchiveFile(r.PathValue("id"), r.PathValue("fileId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

func (s *Server) handleMeCalendar(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	token, err := s.Store.EnsureCalendarToken(user.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	url := requestBaseURL(r) + "/calendar/" + token + ".ics"
	webcal := "webcal://" + strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	writeJSON(w, http.StatusOK, map[string]any{"url": url, "webcalUrl": webcal})
}

func (s *Server) handleCalendarFeed(w http.ResponseWriter, r *http.Request) {
	user, err := s.Store.UserByCalendarToken(strings.TrimSuffix(r.PathValue("token"), ".ics"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	dates, err := s.Store.ListAcceptedDatesForRole(user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	body := store.RenderICS("New Spirit", dates)
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="newspirit.ics"`)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func (s *Server) handleDirectory(w http.ResponseWriter, r *http.Request) {
	if _, err := s.userFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	list, err := s.Store.ListDirectory()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": list})
}

func (s *Server) handleProposals(w http.ResponseWriter, r *http.Request) {
	if _, err := s.userFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	list, err := s.Store.ListProposals()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposals": list})
}

func (s *Server) handleCreateProposal(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.CreateProposal(user.ID, body.Title, body.URL)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"proposal": p})
}

func (s *Server) handleControllerUpdateProposal(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Status  *string `json:"status"`
		Comment *string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.UpdateProposal(r.PathValue("id"), body.Status, body.Comment)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"proposal": p})
}

func (s *Server) handleControllerDeleteProposal(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	if err := s.Store.DeleteProposal(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleControllerChannel(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var n int
	if _, err := fmt.Sscanf(r.PathValue("n"), "%d", &n); err != nil {
		writeError(w, http.StatusBadRequest, "unknown channel")
		return
	}
	var body struct {
		UserID  string `json:"userId"`
		Comment string `json:"comment"`
		V48     bool   `json:"v48"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ch, err := s.Store.SetChannel(n, body.UserID, body.Comment, body.V48)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"channel": ch})
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
	_ = s.Store.TouchLastConnected(user.ID)
	s.publishOnline()
	defer func() {
		if s.Hub.StopStream(user.ID) {
			s.Hub.Broadcast(hub.Envelope{Type: "streamEnded"})
		}
		if peer := s.clearCall(user.ID); peer != "" {
			s.Hub.SendToUser(peer, hub.Envelope{
				Type: "callHangup",
				Data: map[string]any{"from": user.ID, "nickname": user.Nickname},
			})
		}
		s.Hub.UnregisterMember(c)
		s.publishOnline()
	}()
	s.readMemberLoop(conn, user)
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

func (s *Server) serveManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	serveFSFile(w, r, s.ClientFS, "manifest.webmanifest")
}

func (s *Server) serveServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Service-Worker-Allowed", "/")
	w.Header().Set("Cache-Control", "no-cache")
	serveFSFile(w, r, s.ClientFS, "sw.js")
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

func (s *Server) onlinePeople() []store.OnlinePerson {
	people, err := s.Store.PeopleByIDs(s.Hub.OnlineUserIDs())
	if err != nil {
		return []store.OnlinePerson{}
	}
	return people
}

func (s *Server) publishOnline() {
	s.Hub.Broadcast(hub.Envelope{Type: "online", Data: s.onlinePeople()})
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
