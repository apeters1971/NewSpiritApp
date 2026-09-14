package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/apeters/newspirit/internal/store"
)

func postDropboxFile(t *testing.T, c *http.Client, url, name string, data []byte) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func TestArchiveDropboxMemberAndArchiver(t *testing.T) {
	ts, st := plannerTestServer(t)
	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", store.RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	ben, err := st.CreateUser("Ben", "ben@example.com", "secret1", store.RoleChoir, "Alt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(ada.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(ben.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	adaC := memberClient(t, ts, "ada@example.com", "secret2")
	benC := memberClient(t, ts, "ben@example.com", "secret2")

	status, created := postDropboxFile(t, adaC, ts.URL+"/api/archive/dropbox", "Amazing_Grace.txt", []byte("lyrics"))
	item, _ := created["item"].(map[string]any)
	if status != http.StatusCreated || item["name"] != "Amazing_Grace.txt" || item["kind"] != store.ArchiveKindLyrics {
		t.Fatalf("deposit %d %+v", status, created)
	}
	id, _ := item["id"].(string)

	status, listed := doJSON(t, adaC, http.MethodGet, ts.URL+"/api/archive/dropbox", nil)
	items, _ := listed["items"].([]any)
	if status != http.StatusOK || len(items) != 1 {
		t.Fatalf("ada list %d %+v", status, listed)
	}
	status, listed = doJSON(t, benC, http.MethodGet, ts.URL+"/api/archive/dropbox", nil)
	items, _ = listed["items"].([]any)
	if status != http.StatusOK || len(items) != 0 {
		t.Fatalf("ben list %d %+v", status, listed)
	}

	status, _ = doJSON(t, adaC, http.MethodPost, ts.URL+"/api/archive/dropbox/"+id+"/auto", map[string]any{})
	if status != http.StatusForbidden {
		t.Fatalf("member auto %d", status)
	}
	status, _ = doJSON(t, adaC, http.MethodPost, ts.URL+"/api/archive/dropbox/"+id+"/import", map[string]any{"title": "Amazing Grace"})
	if status != http.StatusForbidden {
		t.Fatalf("member import %d", status)
	}

	if _, err := st.SetUserArchiver(ada.ID, true); err != nil {
		t.Fatal(err)
	}
	adaC = memberClient(t, ts, "ada@example.com", "secret2")
	status, auto := doJSON(t, adaC, http.MethodPost, ts.URL+"/api/archive/dropbox/"+id+"/auto", map[string]any{})
	sug, _ := auto["suggestion"].(map[string]any)
	if status != http.StatusOK || sug["title"] != "Amazing Grace" {
		t.Fatalf("auto %d %+v", status, auto)
	}
	status, imported := doJSON(t, adaC, http.MethodPost, ts.URL+"/api/archive/dropbox/"+id+"/import", map[string]any{
		"title": "Amazing Grace", "author": "Newton", "instrument": "choir",
	})
	song, _ := imported["item"].(map[string]any)
	if status != http.StatusOK || song["title"] != "Amazing Grace" || song["composer"] != "Newton" {
		t.Fatalf("import %d %+v", status, imported)
	}
	status, listed = doJSON(t, adaC, http.MethodGet, ts.URL+"/api/archive/dropbox", nil)
	items, _ = listed["items"].([]any)
	if status != http.StatusOK || len(items) != 0 {
		t.Fatalf("empty after import %d %+v", status, listed)
	}
}

func TestControllerDropbox(t *testing.T) {
	ts, _ := plannerTestServer(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{"secret": "dev-secret"})
	res, err := c.Post(ts.URL+"/api/controller/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("controller login %d", res.StatusCode)
	}
	status, created := postDropboxFile(t, c, ts.URL+"/api/controller/archive/dropbox", "Keep_Me_Near.txt", []byte("words"))
	item, _ := created["item"].(map[string]any)
	if status != http.StatusCreated || item["name"] != "Keep_Me_Near.txt" {
		t.Fatalf("ctrl deposit %d %+v", status, created)
	}
	id, _ := item["id"].(string)
	status, imported := doJSON(t, c, http.MethodPost, ts.URL+"/api/controller/archive/dropbox/"+id+"/import", map[string]any{"title": "Keep Me Near"})
	if status != http.StatusOK {
		t.Fatalf("ctrl import %d %+v", status, imported)
	}
}

func TestControllerDropboxAttach(t *testing.T) {
	ts, st := plannerTestServer(t)
	song, err := st.CreateArchiveItem("Oh Happy Day", "Edwin Hawkins")
	if err != nil {
		t.Fatal(err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{"secret": "dev-secret"})
	res, err := c.Post(ts.URL+"/api/controller/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("controller login %d", res.StatusCode)
	}
	status, created := postDropboxFile(t, c, ts.URL+"/api/controller/archive/dropbox", "choir.txt", []byte("lyrics"))
	item, _ := created["item"].(map[string]any)
	if status != http.StatusCreated {
		t.Fatalf("deposit %d %+v", status, created)
	}
	id, _ := item["id"].(string)
	status, attached := doJSON(t, c, http.MethodPost, ts.URL+"/api/controller/archive/dropbox/"+id+"/attach", map[string]any{
		"itemId": song.ID, "instrument": "choir",
	})
	got, _ := attached["item"].(map[string]any)
	if status != http.StatusOK || got["title"] != "Oh Happy Day" {
		t.Fatalf("attach %d %+v", status, attached)
	}
	files, _ := got["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("files %+v", files)
	}
}
