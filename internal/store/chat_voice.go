package store

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	ChatKindText    = "text"
	ChatKindVoice   = "voice"
	ChatVoiceMax    = 8 << 20
	ChatVoiceMaxMs  = 120000
	chatVoiceMinLen = 64
)

func (s *Store) migrateChatVoice() error {
	_, _ = s.db.Exec(`ALTER TABLE chat_messages ADD COLUMN kind TEXT NOT NULL DEFAULT 'text'`)
	_, _ = s.db.Exec(`ALTER TABLE chat_messages ADD COLUMN mime TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE chat_messages ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0`)
	if s.chatDir() == "" {
		return nil
	}
	return os.MkdirAll(s.chatDir(), 0o755)
}

func (s *Store) chatDir() string {
	if s.mediaDir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.mediaDir), "chat")
}

func (s *Store) chatVoicePath(id string) string {
	return filepath.Join(s.chatDir(), id)
}

func (s *Store) removeChatVoiceFile(id string) {
	if id == "" || s.chatDir() == "" {
		return
	}
	_ = os.Remove(s.chatVoicePath(id))
}

func (s *Store) AddChatVoice(userID, room, filename string, r io.Reader, durationMs int) (ChatMessage, error) {
	u, err := s.UserByID(userID)
	if err != nil {
		return ChatMessage{}, err
	}
	if err := s.resolveChatRoom(room, u.Role, true); err != nil {
		return ChatMessage{}, err
	}
	return s.addChatVoice(u, room, filename, r, durationMs)
}

func (s *Store) AddAdminChatVoice(room, filename string, r io.Reader, durationMs int) (ChatMessage, error) {
	if err := s.resolveChatRoom(room, "", true); err != nil {
		return ChatMessage{}, err
	}
	return s.addChatVoice(User{}, room, filename, r, durationMs)
}

func (s *Store) addChatVoice(u User, room, filename string, r io.Reader, durationMs int) (ChatMessage, error) {
	if r == nil {
		return ChatMessage{}, fmt.Errorf("file is required")
	}
	if durationMs < 0 {
		durationMs = 0
	}
	if durationMs > ChatVoiceMaxMs+2000 {
		return ChatMessage{}, fmt.Errorf("voice is too long")
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		head = head[:n]
	} else if err != nil {
		return ChatMessage{}, fmt.Errorf("file is required")
	} else {
		head = head[:n]
	}
	if len(head) < chatVoiceMinLen {
		return ChatMessage{}, fmt.Errorf("voice is too short")
	}
	mime, err := sniffChatVoice(filename, head)
	if err != nil {
		return ChatMessage{}, err
	}
	id := newID()
	dir := s.chatDir()
	if dir == "" {
		return ChatMessage{}, fmt.Errorf("file is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ChatMessage{}, err
	}
	dest := s.chatVoicePath(id)
	f, err := os.Create(dest)
	if err != nil {
		return ChatMessage{}, err
	}
	written, copyErr := io.Copy(f, io.MultiReader(bytes.NewReader(head), io.LimitReader(r, int64(ChatVoiceMax)-int64(len(head))+1)))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return ChatMessage{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return ChatMessage{}, closeErr
	}
	if written > int64(ChatVoiceMax) {
		_ = os.Remove(dest)
		return ChatMessage{}, fmt.Errorf("file is too large")
	}
	msg := ChatMessage{
		ID:             id,
		Room:           room,
		UserID:         u.ID,
		Nickname:       u.Nickname,
		HasPhoto:       u.HasPhoto,
		PhotoUpdatedAt: u.PhotoUpdatedAt,
		Kind:           ChatKindVoice,
		MIME:           mime,
		DurationMs:     durationMs,
		CreatedAt:      now(),
		Reactions:      []ChatReaction{},
	}
	if u.ID == "" {
		msg.IsAdmin = true
		msg.Nickname = s.AdminAlias()
	}
	var uid any
	if u.ID != "" {
		uid = u.ID
	}
	if _, err := s.db.Exec(
		`INSERT INTO chat_messages(id, room, user_id, text, created_at, kind, mime, duration_ms) VALUES(?,?,?,?,?,?,?,?)`,
		msg.ID, msg.Room, uid, "", fmtTime(msg.CreatedAt), msg.Kind, msg.MIME, msg.DurationMs,
	); err != nil {
		_ = os.Remove(dest)
		return ChatMessage{}, err
	}
	return msg, nil
}

func (s *Store) ChatVoiceFile(role, room, messageID string) (ChatMessage, string, error) {
	if err := s.resolveChatRoom(room, role, false); err != nil {
		return ChatMessage{}, "", err
	}
	return s.chatVoiceFile(room, messageID)
}

func (s *Store) ChatVoiceFileForRoom(room, messageID string) (ChatMessage, string, error) {
	if err := s.resolveChatRoom(room, "", false); err != nil {
		return ChatMessage{}, "", err
	}
	return s.chatVoiceFile(room, messageID)
}

func (s *Store) chatVoiceFile(room, messageID string) (ChatMessage, string, error) {
	m, err := s.getChatMessage(messageID, chatReactionActor, true)
	if err != nil {
		return ChatMessage{}, "", err
	}
	if m.Room != room || m.Kind != ChatKindVoice {
		return ChatMessage{}, "", ErrNotFound
	}
	p := s.chatVoicePath(m.ID)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return ChatMessage{}, "", ErrNotFound
		}
		return ChatMessage{}, "", err
	}
	return m, p, nil
}

func sniffChatVoice(filename string, data []byte) (string, error) {
	ext := strings.ToLower(path.Ext(filename))
	if len(data) >= 4 && data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		return "audio/webm", nil
	}
	if len(data) >= 4 && string(data[:4]) == "OggS" {
		return "audio/ogg", nil
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return "audio/wav", nil
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		return "audio/mp4", nil
	}
	if len(data) >= 3 && string(data[:3]) == "ID3" {
		return "audio/mpeg", nil
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1]&0xE0 == 0xE0 {
		return "audio/mpeg", nil
	}
	detected := http.DetectContentType(data)
	if i := strings.IndexByte(detected, ';'); i >= 0 {
		detected = strings.TrimSpace(detected[:i])
	}
	switch {
	case ext == ".webm" || detected == "audio/webm" || detected == "video/webm":
		return "audio/webm", nil
	case ext == ".ogg" || detected == "audio/ogg":
		return "audio/ogg", nil
	case ext == ".wav" || detected == "audio/wav" || detected == "audio/x-wav" || detected == "audio/wave":
		return "audio/wav", nil
	case ext == ".m4a" || ext == ".mp4" || detected == "audio/mp4":
		return "audio/mp4", nil
	case ext == ".mp3" || detected == "audio/mpeg":
		return "audio/mpeg", nil
	case strings.HasPrefix(detected, "audio/"):
		return detected, nil
	default:
		return "", fmt.Errorf("file type is not allowed")
	}
}
