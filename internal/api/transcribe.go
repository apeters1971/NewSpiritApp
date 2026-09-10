package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
)

const cloudflareWhisperModel = "@cf/openai/whisper-large-v3-turbo"

var cloudflareHTTP = &http.Client{Timeout: 60 * time.Second}

func (s *Server) queueVoiceTranscript(msg store.ChatMessage) {
	if strings.TrimSpace(s.CloudflareToken) == "" || strings.TrimSpace(s.CloudflareAccountID) == "" {
		return
	}
	go s.transcribeVoice(msg)
}

func (s *Server) transcribeVoice(msg store.ChatMessage) {
	_, path, err := s.Store.ChatVoiceFileForRoom(msg.Room, msg.ID)
	if err != nil {
		log.Printf("voice transcript: load %s: %v", msg.ID, err)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("voice transcript: read %s: %v", msg.ID, err)
		return
	}
	text, err := transcribeCloudflare(s.CloudflareAccountID, s.CloudflareToken, data)
	if err != nil {
		log.Printf("voice transcript: %s: %v", msg.ID, err)
		return
	}
	updated, err := s.Store.SetChatVoiceText(msg.ID, text)
	if err != nil {
		log.Printf("voice transcript: save %s: %v", msg.ID, err)
		return
	}
	s.publishChat(updated.Room, hub.Envelope{Type: "chatUpdate", Data: updated})
}

func transcribeCloudflare(accountID, token string, audio []byte) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("audio is empty")
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", accountID, cloudflareWhisperModel)
	text, err := cloudflareWhisper(url, token, audio, "application/octet-stream")
	if err == nil {
		return text, nil
	}
	body, _ := json.Marshal(map[string]string{"audio": base64.StdEncoding.EncodeToString(audio)})
	return cloudflareWhisper(url, token, body, "application/json")
}

func cloudflareWhisper(url, token string, body []byte, contentType string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)
	res, err := cloudflareHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", err
	}
	text, err := parseCloudflareTranscript(payload)
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

func parseCloudflareTranscript(payload []byte) (string, error) {
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
	text := transcriptFromResult(wrap.Result)
	if text == "" {
		return "", fmt.Errorf("empty transcript")
	}
	return text, nil
}

func transcriptFromResult(raw json.RawMessage) string {
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
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return strings.TrimSpace(obj.Text)
	}
	return ""
}

func ResolveCloudflareAccountID(token, hint string) (string, error) {
	hint = strings.TrimSpace(hint)
	if hint != "" {
		return hint, nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("CLOUDFLARE_API_TOKEN is empty")
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.cloudflare.com/client/v4/accounts?per_page=50", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := cloudflareHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var wrap struct {
		Success bool `json:"success"`
		Result  []struct {
			ID string `json:"id"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(payload, &wrap); err != nil {
		return "", fmt.Errorf("invalid cloudflare accounts response")
	}
	if !wrap.Success {
		msg := "could not list accounts"
		if len(wrap.Errors) > 0 && wrap.Errors[0].Message != "" {
			msg = wrap.Errors[0].Message
		}
		return "", fmt.Errorf("%s", msg)
	}
	if len(wrap.Result) == 0 || strings.TrimSpace(wrap.Result[0].ID) == "" {
		return "", fmt.Errorf("no Cloudflare account on this token")
	}
	return wrap.Result[0].ID, nil
}
