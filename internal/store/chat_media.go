package store

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

const (
	ChatKindImage = "image"
	ChatKindVideo = "video"
	ChatImageMax  = 8 << 20
	ChatVideoMax  = 64 << 20
	chatMediaMin  = 16
)

func ChatMediaMax(kind string) int {
	if kind == ChatKindVideo {
		return ChatVideoMax
	}
	return ChatImageMax
}

func (s *Store) AddChatMedia(userID, room, filename string, r io.Reader) (ChatMessage, error) {
	u, err := s.UserByID(userID)
	if err != nil {
		return ChatMessage{}, err
	}
	if err := s.resolveChatRoom(room, u.Role, true); err != nil {
		return ChatMessage{}, err
	}
	return s.addChatMedia(u, room, filename, r)
}

func (s *Store) AddAdminChatMedia(room, filename string, r io.Reader) (ChatMessage, error) {
	if err := s.resolveChatRoom(room, "", true); err != nil {
		return ChatMessage{}, err
	}
	return s.addChatMedia(User{}, room, filename, r)
}

func (s *Store) addChatMedia(u User, room, filename string, r io.Reader) (ChatMessage, error) {
	if r == nil {
		return ChatMessage{}, fmt.Errorf("file is required")
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
	if len(head) < chatMediaMin {
		return ChatMessage{}, fmt.Errorf("file is required")
	}
	kind, mime, err := sniffChatMedia(filename, head)
	if err != nil {
		return ChatMessage{}, err
	}
	max := ChatMediaMax(kind)
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
	written, copyErr := io.Copy(f, io.MultiReader(bytes.NewReader(head), io.LimitReader(r, int64(max)-int64(len(head))+1)))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return ChatMessage{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dest)
		return ChatMessage{}, closeErr
	}
	if written > int64(max) {
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
		Kind:           kind,
		MIME:           mime,
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
		msg.ID, msg.Room, uid, "", fmtTime(msg.CreatedAt), msg.Kind, msg.MIME, 0,
	); err != nil {
		_ = os.Remove(dest)
		return ChatMessage{}, err
	}
	return msg, nil
}

func (s *Store) ChatMediaFile(role, room, messageID string) (ChatMessage, string, error) {
	if err := s.resolveChatRoom(room, role, false); err != nil {
		return ChatMessage{}, "", err
	}
	return s.chatMediaFile(room, messageID)
}

func (s *Store) ChatMediaFileForRoom(room, messageID string) (ChatMessage, string, error) {
	if err := s.resolveChatRoom(room, "", false); err != nil {
		return ChatMessage{}, "", err
	}
	return s.chatMediaFile(room, messageID)
}

func (s *Store) chatMediaFile(room, messageID string) (ChatMessage, string, error) {
	m, err := s.getChatMessage(messageID, chatReactionActor, true)
	if err != nil {
		return ChatMessage{}, "", err
	}
	if m.Room != room || (m.Kind != ChatKindImage && m.Kind != ChatKindVideo) {
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

func sniffChatMedia(filename string, data []byte) (kind, mime string, err error) {
	ext := strings.ToLower(path.Ext(filename))
	detected := http.DetectContentType(data)
	if i := strings.IndexByte(detected, ';'); i >= 0 {
		detected = strings.TrimSpace(detected[:i])
	}
	switch {
	case ext == ".jpg" || ext == ".jpeg" || detected == "image/jpeg":
		return ChatKindImage, "image/jpeg", nil
	case ext == ".png" || detected == "image/png":
		return ChatKindImage, "image/png", nil
	case ext == ".webp" || detected == "image/webp":
		return ChatKindImage, "image/webp", nil
	case ext == ".gif" || detected == "image/gif":
		return ChatKindImage, "image/gif", nil
	case ext == ".mp4" || ext == ".m4v" || detected == "video/mp4":
		return ChatKindVideo, "video/mp4", nil
	case ext == ".webm" || detected == "video/webm":
		return ChatKindVideo, "video/webm", nil
	case ext == ".mov" || detected == "video/quicktime":
		return ChatKindVideo, "video/quicktime", nil
	case strings.HasPrefix(detected, "image/"):
		return ChatKindImage, detected, nil
	case strings.HasPrefix(detected, "video/"):
		return ChatKindVideo, detected, nil
	default:
		return "", "", fmt.Errorf("file type is not allowed")
	}
}
