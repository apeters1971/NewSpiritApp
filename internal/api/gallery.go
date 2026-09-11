package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
)

func (s *Server) handleGalleryList(w http.ResponseWriter, r *http.Request) {
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
	items, err := s.Store.ListGallery(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"gallery": items})
}

func (s *Server) handleGalleryUpload(w http.ResponseWriter, r *http.Request) {
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
	name, f, err := s.readGalleryUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()
	item, err := s.Store.AddGalleryItemFromReader(id, user.ID, name, f)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleGalleryFile(w http.ResponseWriter, r *http.Request) {
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
	item, path, err := s.Store.GalleryFilePath(id, r.PathValue("fileId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeGalleryFile(w, r, item, path)
}

func (s *Server) handleControllerGalleryList(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	items, err := s.Store.ListGallery(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"gallery": items})
}

func (s *Server) handleControllerGalleryUpload(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	name, f, err := s.readGalleryUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()
	item, err := s.Store.AddGalleryItemFromReader(r.PathValue("id"), "", name, f)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleControllerGalleryFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	item, path, err := s.Store.GalleryFilePath(r.PathValue("id"), r.PathValue("fileId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeGalleryFile(w, r, item, path)
}

func (s *Server) handleControllerGalleryDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	if err := s.Store.DeleteGalleryItem(r.PathValue("id"), r.PathValue("fileId"), "", true); err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) readGalleryUpload(w http.ResponseWriter, r *http.Request) (string, io.ReadCloser, error) {
	max := int64(store.GalleryMaxBytes()) + 512<<10
	r.Body = http.MaxBytesReader(w, r.Body, max)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
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

func (s *Server) handleGalleryHub(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	albums, err := s.Store.ListGalleryHub(user)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"albums": albums})
}

func (s *Server) handleAlbumList(w http.ResponseWriter, r *http.Request) {
	if _, err := s.userFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	album := r.PathValue("album")
	yearQ := r.URL.Query().Get("year")
	monthQ := r.URL.Query().Get("month")
	if yearQ == "" && monthQ == "" {
		months, err := s.Store.ListAlbumMonths(album)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"months": months})
		return
	}
	year, yearErr := strconv.Atoi(yearQ)
	month, monthErr := strconv.Atoi(monthQ)
	if yearErr != nil || monthErr != nil {
		writeError(w, http.StatusBadRequest, "invalid month")
		return
	}
	items, err := s.Store.ListAlbumMonth(album, year, month)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"gallery": items})
}

func (s *Server) handleAlbumUpload(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	album := r.PathValue("album")
	if !store.ValidAlbum(album) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	name, f, err := s.readGalleryUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()
	item, err := s.Store.AddAlbumItemFromReader(album, user.ID, name, f)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleAlbumFile(w http.ResponseWriter, r *http.Request) {
	if _, err := s.userFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	item, path, err := s.Store.AlbumFilePath(r.PathValue("album"), r.PathValue("fileId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeGalleryFile(w, r, item, path)
}

func writeGalleryFile(w http.ResponseWriter, r *http.Request, item store.GalleryItem, path string) {
	w.Header().Set("Content-Type", item.MIME)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	name := item.Name
	if name == "" {
		name = "media"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, name))
	http.ServeFile(w, r, path)
}
