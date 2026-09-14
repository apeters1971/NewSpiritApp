package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/apeters/newspirit/internal/store"
)

func TestInferArchiveAutoKind(t *testing.T) {
	if got := inferArchiveAutoKind("Oh_Happy_Day.mp3", []byte("ID3")); got != store.ArchiveKindAudio {
		t.Fatalf("mp3 %q", got)
	}
	if got := inferArchiveAutoKind("sheet.pdf", []byte("%PDF-1.4")); got != store.ArchiveKindSheet {
		t.Fatalf("pdf %q", got)
	}
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if got := inferArchiveAutoKind("page.png", png); got != store.ArchiveKindSheet {
		t.Fatalf("png %q", got)
	}
	if got := inferArchiveAutoKind("notes.txt", []byte("Amazing Grace")); got != store.ArchiveKindLyrics {
		t.Fatalf("txt %q", got)
	}
	if got := inferArchiveAutoKind("x.doc", []byte("not music")); got != "" {
		t.Fatalf("bad %q", got)
	}
}

func TestTitleFromFilename(t *testing.T) {
	if got := titleFromFilename("Oh_Happy_Day.mp3"); got != "Oh Happy Day" {
		t.Fatalf("got %q", got)
	}
	if got := titleFromFilename("amazing-grace.pdf"); got != "amazing grace" {
		t.Fatalf("got %q", got)
	}
}

func TestParseArchiveAutoJSON(t *testing.T) {
	title, author, inst := parseArchiveAutoJSON("```json\n{\"title\":\"Oh Happy Day\",\"author\":\"Edwin Hawkins\",\"instrument\":\"choir\"}\n```")
	if title != "Oh Happy Day" || author != "Edwin Hawkins" || inst != "choir" {
		t.Fatalf("%q %q %q", title, author, inst)
	}
	title, author, inst = parseArchiveAutoJSON(`{"title":"X","composer":"Y","instrument":"piano"}`)
	if title != "X" || author != "Y" || inst != "piano" {
		t.Fatalf("composer alias %q %q %q", title, author, inst)
	}
	if t2, a2, i2 := parseArchiveAutoJSON("no json here"); t2 != "" || a2 != "" || i2 != "" {
		t.Fatalf("empty %q %q %q", t2, a2, i2)
	}
}

func TestCleanArchiveInstrument(t *testing.T) {
	if got := cleanArchiveInstrument("Sopran"); got != "soprano" {
		t.Fatalf("sopran %q", got)
	}
	if got := cleanArchiveInstrument("choir"); got != "choir" {
		t.Fatalf("choir %q", got)
	}
	if got := cleanArchiveInstrument("  Piano!!  "); got != "piano" {
		t.Fatalf("piano %q", got)
	}
}

func TestMergeAndFilenameGuess(t *testing.T) {
	base := filenameArchiveGuess("Keep_Me.mp3", store.ArchiveKindAudio)
	if base.Title != "Keep Me" || base.Kind != store.ArchiveKindAudio || base.Name != "Keep_Me.mp3" {
		t.Fatalf("%+v", base)
	}
	got := mergeArchiveAuto(base, archiveAutoSuggestion{Title: "Keep Me Near", Author: "Ada", Instrument: "choir"})
	if got.Title != "Keep Me Near" || got.Author != "Ada" || got.Instrument != "choir" || got.Kind != store.ArchiveKindAudio {
		t.Fatalf("%+v", got)
	}
}

func TestExtractPDFStrings(t *testing.T) {
	got := extractPDFStrings([]byte("BT /F1 12 Tf (Oh Happy Day) Tj ET"))
	if !strings.Contains(got, "Oh Happy Day") {
		t.Fatalf("got %q", got)
	}
}

func TestArchiveAutoFilenameOnly(t *testing.T) {
	ts, st := plannerTestServer(t)
	ada, err := st.CreateUser("Ada", "ada@example.com", "secret1", store.RoleChoir, "Sopran")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeOwnPassword(ada.ID, "secret2"); err != nil {
		t.Fatal(err)
	}
	c := memberClient(t, ts, "ada@example.com", "secret2")
	status, cat := doJSON(t, c, http.MethodGet, ts.URL+"/api/catalog", nil)
	if status != http.StatusOK || cat["archiveAuto"] != false {
		t.Fatalf("catalog %+v %d", cat, status)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "Oh_Happy_Day.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("ID3fake")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/archive/auto", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out archiveAutoSuggestion
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || out.Title != "Oh Happy Day" || out.Kind != store.ArchiveKindAudio || out.Name != "Oh_Happy_Day.mp3" {
		t.Fatalf("%d %+v", res.StatusCode, out)
	}
}

func TestParseCloudflareInstruct(t *testing.T) {
	text, err := parseCloudflareInstruct([]byte(`{"success":true,"result":{"response":"{\"title\":\"A\"}"}}`))
	if err != nil || text != `{"title":"A"}` {
		t.Fatalf("%q %v", text, err)
	}
	if _, err := parseCloudflareInstruct([]byte(`{"success":false,"errors":[{"message":"nope"}]}`)); err == nil {
		t.Fatal("expected error")
	}
}
