package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
)

type promoTicketBody struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (s *Server) handlePromoList(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := r.PathValue("id")
	if err := s.Store.MemberCanSeeDate(user, id); err != nil {
		writeStoreError(w, err)
		return
	}
	items, err := s.Store.ListPromo(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	note, err := s.Store.PromoNote(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promo": items, "note": note})
}

func (s *Server) handlePromoFile(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := r.PathValue("id")
	if err := s.Store.MemberCanSeeDate(user, id); err != nil {
		writeStoreError(w, err)
		return
	}
	item, path, err := s.Store.PromoFilePath(id, r.PathValue("fileId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writePromoFile(w, r, item, path)
}

func (s *Server) handleControllerPromoList(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	id := r.PathValue("id")
	items, err := s.Store.ListPromo(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	note, err := s.Store.PromoNote(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promo": items, "note": note})
}

func (s *Server) handleControllerPromoAdd(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	id := r.PathValue("id")
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
		item, err := s.addControllerPromoFile(w, r, id)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, http.StatusBadRequest, "file is too large")
				return
			}
			writeStoreError(w, err)
			return
		}
		s.Hub.Broadcast(hub.Envelope{Type: "changed"})
		writeJSON(w, http.StatusCreated, map[string]any{"item": item})
		return
	}
	var body promoTicketBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Kind != "" && body.Kind != store.PromoKindTicket {
		writeError(w, http.StatusBadRequest, "unknown promo kind")
		return
	}
	item, err := s.Store.AddPromoTicket(id, body.Name, body.URL)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleControllerPromoNote(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	note, err := s.Store.SetPromoNote(r.PathValue("id"), body.Note)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"note": note})
}

func (s *Server) addControllerPromoFile(w http.ResponseWriter, r *http.Request, dateID string) (store.PromoItem, error) {
	kind, name, f, err := s.readPromoUpload(w, r)
	if err != nil {
		return store.PromoItem{}, err
	}
	defer f.Close()
	return s.Store.AddPromoFile(dateID, kind, name, f)
}

func (s *Server) handleControllerPromoFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	item, path, err := s.Store.PromoFilePath(r.PathValue("id"), r.PathValue("fileId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writePromoFile(w, r, item, path)
}

func (s *Server) handleControllerPromoDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	if err := s.Store.DeletePromoItem(r.PathValue("id"), r.PathValue("fileId")); err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) readPromoUpload(w http.ResponseWriter, r *http.Request) (string, string, io.ReadCloser, error) {
	max := int64(store.PromoMaxBytes) + 512<<10
	r.Body = http.MaxBytesReader(w, r.Body, max)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return "", "", nil, fmt.Errorf("file is too large")
	}
	kind := strings.TrimSpace(r.FormValue("kind"))
	if !store.ValidPromoFileKind(kind) {
		return "", "", nil, fmt.Errorf("unknown promo kind")
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return "", "", nil, fmt.Errorf("file is required")
	}
	name := ""
	if hdr != nil {
		name = hdr.Filename
	}
	return kind, name, f, nil
}

func writePromoFile(w http.ResponseWriter, r *http.Request, item store.PromoItem, path string) {
	w.Header().Set("Content-Type", item.MIME)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	name := item.Name
	if name == "" {
		name = "file"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, name))
	http.ServeFile(w, r, path)
}
