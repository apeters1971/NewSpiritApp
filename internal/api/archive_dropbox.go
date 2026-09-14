package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/apeters/newspirit/internal/hub"
)

func (s *Server) handleArchiveDropboxList(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	s.writeDropboxList(w, user.Archiver, user.ID)
}

func (s *Server) handleControllerDropboxList(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.writeDropboxList(w, true, "")
}

func (s *Server) writeDropboxList(w http.ResponseWriter, all bool, userID string) {
	items, err := s.Store.ListDropbox(all, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleArchiveDropboxAdd(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	s.addDropboxUpload(w, r, user.ID)
}

func (s *Server) handleControllerDropboxAdd(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.addDropboxUpload(w, r, "")
}

func (s *Server) addDropboxUpload(w http.ResponseWriter, r *http.Request, userID string) {
	name, data, err := s.readArchiveAutoUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.Store.AddDropboxFile(name, userID, data)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleArchiveDropboxPatch(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !user.Archiver {
		writeError(w, http.StatusForbidden, "only an archiver can manage the import queue")
		return
	}
	s.patchDropbox(w, r)
}

func (s *Server) handleControllerDropboxPatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.patchDropbox(w, r)
}

func (s *Server) patchDropbox(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title      string `json:"title"`
		Author     string `json:"author"`
		Instrument string `json:"instrument"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := s.Store.UpdateDropboxMeta(r.PathValue("id"), body.Title, body.Author, body.Instrument)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleArchiveDropboxDelete(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	item, err := s.Store.DropboxItem(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !s.Store.CanManageDropbox(user, item) {
		writeError(w, http.StatusForbidden, "only an archiver can manage the import queue")
		return
	}
	s.deleteDropbox(w, item.ID)
}

func (s *Server) handleControllerDropboxDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.deleteDropbox(w, r.PathValue("id"))
}

func (s *Server) deleteDropbox(w http.ResponseWriter, id string) {
	if err := s.Store.DeleteDropbox(id); err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleArchiveDropboxAuto(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !user.Archiver {
		writeError(w, http.StatusForbidden, "only an archiver can manage the import queue")
		return
	}
	s.autoDropbox(w, r.PathValue("id"))
}

func (s *Server) handleControllerDropboxAuto(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.autoDropbox(w, r.PathValue("id"))
}

func (s *Server) autoDropbox(w http.ResponseWriter, id string) {
	item, err := s.Store.DropboxItem(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := filenameArchiveGuess(item.Name, item.Kind)
	if s.archiveAutoEnabled() {
		if guessed, err := s.analyzeArchiveAuto(item.Name, item.Kind, item.Data); err == nil {
			out = mergeArchiveAuto(out, guessed)
		}
	}
	updated, err := s.Store.UpdateDropboxMeta(item.ID, out.Title, out.Author, out.Instrument)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out.Title = updated.Title
	out.Author = updated.Author
	out.Instrument = updated.Instrument
	out.Kind = updated.Kind
	out.Name = updated.Name
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": updated, "suggestion": out})
}

func (s *Server) handleArchiveDropboxImport(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !user.Archiver {
		writeError(w, http.StatusForbidden, "only an archiver can manage the import queue")
		return
	}
	s.importDropbox(w, r)
}

func (s *Server) handleControllerDropboxImport(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.importDropbox(w, r)
}

func (s *Server) importDropbox(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title      string `json:"title"`
		Author     string `json:"author"`
		Instrument string `json:"instrument"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
	}
	item, err := s.Store.ImportDropbox(r.PathValue("id"), body.Title, body.Author, body.Instrument)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleArchiveDropboxAttach(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !user.Archiver {
		writeError(w, http.StatusForbidden, "only an archiver can manage the import queue")
		return
	}
	s.attachDropbox(w, r)
}

func (s *Server) handleControllerDropboxAttach(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.attachDropbox(w, r)
}

func (s *Server) attachDropbox(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemID     string `json:"itemId"`
		Instrument string `json:"instrument"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.ItemID) == "" {
		writeError(w, http.StatusBadRequest, "archive item is required")
		return
	}
	item, err := s.Store.AttachDropbox(r.PathValue("id"), body.ItemID, body.Instrument)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}
