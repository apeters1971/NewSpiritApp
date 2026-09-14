package api

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/apeters/newspirit/internal/store"
)

const (
	cloudflareInstructModel = "@cf/meta/llama-3.2-3b-instruct"
	cloudflareVisionModel   = "@cf/meta/llama-3.2-11b-vision-instruct"
	archiveAutoTextMax      = 4000
	archiveAutoRoleMax      = 40
)

type archiveAutoSuggestion struct {
	Title      string `json:"title"`
	Author     string `json:"author"`
	Instrument string `json:"instrument"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
}

func (s *Server) archiveAutoEnabled() bool {
	return strings.TrimSpace(s.CloudflareToken) != "" && strings.TrimSpace(s.CloudflareAccountID) != ""
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	cat := store.Catalog()
	cat["archiveAuto"] = s.archiveAutoEnabled()
	writeJSON(w, http.StatusOK, cat)
}

func (s *Server) handleArchiveAuto(w http.ResponseWriter, r *http.Request) {
	if _, err := s.userFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	s.serveArchiveAuto(w, r)
}

func (s *Server) handleControllerArchiveAuto(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.serveArchiveAuto(w, r)
}

func (s *Server) serveArchiveAuto(w http.ResponseWriter, r *http.Request) {
	name, data, err := s.readArchiveAutoUpload(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	kind := inferArchiveAutoKind(name, data)
	if kind == "" {
		writeError(w, http.StatusBadRequest, "file type is not allowed")
		return
	}
	out := filenameArchiveGuess(name, kind)
	if s.archiveAutoEnabled() {
		if guessed, err := s.analyzeArchiveAuto(name, kind, data); err == nil {
			out = mergeArchiveAuto(out, guessed)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) readArchiveAutoUpload(w http.ResponseWriter, r *http.Request) (string, []byte, error) {
	max := int64(store.ArchiveMaxAudio) + 512<<10
	r.Body = http.MaxBytesReader(w, r.Body, max)
	if err := r.ParseMultipartForm(max); err != nil {
		return "", nil, fmt.Errorf("file is too large")
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return "", nil, fmt.Errorf("file is required")
	}
	defer f.Close()
	limit := int64(store.ArchiveMaxAudio) + 1
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
	if kind := inferArchiveAutoKind(name, raw); kind != "" && int64(len(raw)) > int64(store.ArchiveMaxBytes(kind)) {
		return "", nil, fmt.Errorf("file is too large")
	}
	return name, raw, nil
}

func (s *Server) analyzeArchiveAuto(name, kind string, data []byte) (archiveAutoSuggestion, error) {
	hint := filenameArchiveGuess(name, kind)
	text := ""
	var image []byte
	var imageMIME string
	if store.ArchiveKindIsAudio(kind) {
		if t, err := transcribeCloudflare(s.CloudflareAccountID, s.CloudflareToken, data); err == nil {
			text = t
		}
	} else if kind == store.ArchiveKindSheet {
		text = pdfPlainText(data)
		if text == "" {
			image, imageMIME = firstEmbeddedImage(data)
			if image == nil && isRasterImage(data) {
				image, imageMIME = data, http.DetectContentType(data)
			}
		}
	}
	raw, err := s.cloudflareArchiveLabel(hint.Title, text, image, imageMIME)
	if err != nil {
		return archiveAutoSuggestion{}, err
	}
	title, author, instrument := parseArchiveAutoJSON(raw)
	return archiveAutoSuggestion{
		Title:      cleanArchiveTitle(title),
		Author:     cleanArchiveAuthor(author),
		Instrument: cleanArchiveInstrument(instrument),
		Kind:       kind,
		Name:       path.Base(strings.TrimSpace(name)),
	}, nil
}

func (s *Server) cloudflareArchiveLabel(filename, text string, image []byte, imageMIME string) (string, error) {
	prompt := archiveAutoPrompt(filename, text)
	if len(image) > 0 {
		if out, err := s.cloudflareVisionJSON(prompt, image, imageMIME); err == nil && strings.TrimSpace(out) != "" {
			return out, nil
		}
	}
	return s.cloudflareInstructJSON(prompt)
}

func (s *Server) cloudflareInstructJSON(prompt string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": archiveAutoSystem},
			{"role": "user", "content": prompt},
		},
		"max_tokens": 220,
	})
	return s.cloudflareRun(cloudflareInstructModel, body)
}

func (s *Server) cloudflareVisionJSON(prompt string, image []byte, mime string) (string, error) {
	if mime == "" {
		mime = http.DetectContentType(image)
	}
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}
	body, _ := json.Marshal(map[string]any{
		"image":      base64.StdEncoding.EncodeToString(image),
		"prompt":     archiveAutoSystem + "\n\n" + prompt,
		"max_tokens": 220,
	})
	return s.cloudflareRun(cloudflareVisionModel, body)
}

func (s *Server) cloudflareRun(model string, body []byte) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", s.CloudflareAccountID, model)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.CloudflareToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := cloudflareHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", err
	}
	text, err := parseCloudflareInstruct(payload)
	if err != nil {
		if res.StatusCode >= 300 {
			return "", fmt.Errorf("cloudflare %s: %s", res.Status, strings.TrimSpace(string(payload)))
		}
		return "", err
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("cloudflare %s", res.Status)
	}
	return text, nil
}

const archiveAutoSystem = `You label music for a gospel choir archive.
Return only JSON: {"title":"","author":"","instrument":""}
title is the song name. author is the composer or artist.
instrument is one of: choir, soprano, alto, tenor, bass, piano, guitar, band, orchestra, drums, percussion, hammond, bass guitar, trumpet, sax, trombone, strings, brass, harp.
Use empty strings when unsure.`

func archiveAutoPrompt(filename, text string) string {
	var b strings.Builder
	b.WriteString("Filename: ")
	if strings.TrimSpace(filename) == "" {
		b.WriteString("(none)")
	} else {
		b.WriteString(strings.TrimSpace(filename))
	}
	b.WriteString("\nText:\n")
	text = strings.TrimSpace(text)
	if text == "" {
		b.WriteString("(none)")
	} else {
		if utf8.RuneCountInString(text) > archiveAutoTextMax {
			runes := []rune(text)
			text = string(runes[:archiveAutoTextMax])
		}
		b.WriteString(text)
	}
	return b.String()
}

func inferArchiveAutoKind(name string, data []byte) string {
	return store.InferArchiveKind(name, data)
}

func filenameArchiveGuess(name, kind string) archiveAutoSuggestion {
	base := path.Base(strings.TrimSpace(name))
	return archiveAutoSuggestion{
		Title: titleFromFilename(base),
		Kind:  kind,
		Name:  base,
	}
}

func titleFromFilename(name string) string {
	stem := strings.TrimSuffix(path.Base(name), path.Ext(name))
	stem = strings.ReplaceAll(stem, "_", " ")
	stem = strings.ReplaceAll(stem, "-", " ")
	return strings.Join(strings.Fields(stem), " ")
}

func mergeArchiveAuto(base, extra archiveAutoSuggestion) archiveAutoSuggestion {
	if extra.Title != "" {
		base.Title = extra.Title
	}
	if extra.Author != "" {
		base.Author = extra.Author
	}
	if extra.Instrument != "" {
		base.Instrument = extra.Instrument
	}
	if extra.Kind != "" {
		base.Kind = extra.Kind
	}
	if extra.Name != "" {
		base.Name = extra.Name
	}
	return base
}

func parseArchiveAutoJSON(raw string) (title, author, instrument string) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var obj struct {
		Title      string `json:"title"`
		Author     string `json:"author"`
		Composer   string `json:"composer"`
		Instrument string `json:"instrument"`
	}
	if json.Unmarshal([]byte(raw), &obj) != nil {
		return "", "", ""
	}
	author = obj.Author
	if author == "" {
		author = obj.Composer
	}
	return obj.Title, author, obj.Instrument
}

func parseCloudflareInstruct(payload []byte) (string, error) {
	var wrap struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(payload, &wrap); err != nil {
		return "", fmt.Errorf("invalid cloudflare response")
	}
	if !wrap.Success && len(wrap.Errors) > 0 {
		return "", fmt.Errorf("%s", wrap.Errors[0].Message)
	}
	text := instructFromResult(wrap.Result)
	if text == "" {
		return "", fmt.Errorf("empty model reply")
	}
	return text, nil
}

func instructFromResult(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return strings.TrimSpace(s)
		}
	}
	var obj struct {
		Response string `json:"response"`
		Text     string `json:"text"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		if s := strings.TrimSpace(obj.Response); s != "" {
			return s
		}
		return strings.TrimSpace(obj.Text)
	}
	return ""
}

func cleanArchiveTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > 200 {
		s = string([]rune(s)[:200])
	}
	return s
}

func cleanArchiveAuthor(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > 120 {
		s = string([]rune(s)[:120])
	}
	return s
}

var instrumentAliases = map[string]string{
	"chor":       "choir",
	"sopran":     "soprano",
	"alt":        "alto",
	"tenor/bass": "tenor",
	"tenorbass":  "tenor",
	"e-bass":     "bass guitar",
	"ebass":      "bass guitar",
	"e bass":     "bass guitar",
	"schlagzeug": "drums",
}

var instrumentAllowed = map[string]bool{
	"choir": true, "soprano": true, "alto": true, "tenor": true, "bass": true,
	"piano": true, "guitar": true, "band": true, "orchestra": true, "drums": true,
	"percussion": true, "hammond": true, "bass guitar": true, "trumpet": true,
	"sax": true, "trombone": true, "strings": true, "brass": true, "harp": true,
}

func cleanArchiveInstrument(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	if alias, ok := instrumentAliases[s]; ok {
		s = alias
	}
	if s == "" {
		return ""
	}
	if !instrumentAllowed[s] {
		var b strings.Builder
		for _, r := range s {
			if unicode.IsLetter(r) || r == ' ' || r == '-' || r == '/' {
				b.WriteRune(r)
			}
		}
		s = strings.Join(strings.Fields(b.String()), " ")
	}
	if utf8.RuneCountInString(s) > archiveAutoRoleMax {
		s = string([]rune(s)[:archiveAutoRoleMax])
	}
	return s
}

func isRasterImage(data []byte) bool {
	det := http.DetectContentType(data)
	return strings.HasPrefix(det, "image/png") || strings.HasPrefix(det, "image/jpeg") || strings.HasPrefix(det, "image/webp")
}

func pdfPlainText(data []byte) string {
	if len(data) < 5 || !bytes.HasPrefix(data, []byte("%PDF")) {
		if isRasterImage(data) {
			return ""
		}
		if strings.HasPrefix(http.DetectContentType(data), "text/plain") {
			return string(data)
		}
		return extractPDFStrings(data)
	}
	var parts []string
	for _, stream := range pdfStreams(data) {
		body := maybeInflatePDF(stream)
		if t := extractPDFStrings(body); t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return extractPDFStrings(data)
	}
	return strings.Join(parts, "\n")
}

func pdfStreams(data []byte) [][]byte {
	var out [][]byte
	rest := data
	for {
		i := bytes.Index(rest, []byte("stream"))
		if i < 0 {
			return out
		}
		rest = rest[i+6:]
		if len(rest) >= 2 && rest[0] == '\r' && rest[1] == '\n' {
			rest = rest[2:]
		} else if len(rest) >= 1 && (rest[0] == '\n' || rest[0] == '\r') {
			rest = rest[1:]
		}
		j := bytes.Index(rest, []byte("endstream"))
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j+9:]
		if len(out) >= 40 {
			return out
		}
	}
}

func maybeInflatePDF(raw []byte) []byte {
	raw = bytes.TrimSpace(raw)
	r, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return raw
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil || len(out) == 0 {
		return raw
	}
	return out
}

var pdfStringRe = regexp.MustCompile(`\((?:\\.|[^\\)])*\)`)

func extractPDFStrings(data []byte) string {
	matches := pdfStringRe.FindAll(data, 80)
	var parts []string
	for _, m := range matches {
		s := unescapePDFString(string(m[1 : len(m)-1]))
		s = strings.TrimSpace(s)
		if len(s) < 2 || !utf8.ValidString(s) {
			continue
		}
		letters := 0
		for _, r := range s {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters < 2 {
			continue
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

func unescapePDFString(s string) string {
	s = strings.ReplaceAll(s, `\n`, " ")
	s = strings.ReplaceAll(s, `\r`, " ")
	s = strings.ReplaceAll(s, `\t`, " ")
	s = strings.ReplaceAll(s, `\()`, "(")
	s = strings.ReplaceAll(s, `\)`, ")")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

func firstEmbeddedImage(data []byte) ([]byte, string) {
	if jpeg := extractMarked(data, []byte{0xFF, 0xD8, 0xFF}, []byte{0xFF, 0xD9}); len(jpeg) > 200 {
		return jpeg, "image/jpeg"
	}
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if i := bytes.Index(data, pngSig); i >= 0 {
		end := bytes.Index(data[i:], []byte("IEND"))
		if end > 0 && i+end+8 <= len(data) {
			return data[i : i+end+8], "image/png"
		}
	}
	return nil, ""
}

func extractMarked(data, start, end []byte) []byte {
	i := bytes.Index(data, start)
	if i < 0 {
		return nil
	}
	j := bytes.Index(data[i+len(start):], end)
	if j < 0 {
		return nil
	}
	return data[i : i+len(start)+j+len(end)]
}
