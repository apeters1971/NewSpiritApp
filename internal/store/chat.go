package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	ChatAdminName      = "Admin"
	ChatReadController = "controller"
	EventRoomPrefix    = "event:"
	chatReactionActor  = ""
)

var ChatEmojis = []string{"👍", "❤️", "😂", "😮", "😢", "🎉"}

type ChatReaction struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
	Mine  bool   `json:"mine"`
}

type ChatMessage struct {
	ID             string         `json:"id"`
	Room           string         `json:"room"`
	UserID         string         `json:"userId"`
	Nickname       string         `json:"nickname"`
	IsAdmin        bool           `json:"isAdmin,omitempty"`
	HasPhoto       bool           `json:"hasPhoto"`
	PhotoUpdatedAt *time.Time     `json:"photoUpdatedAt,omitempty"`
	Text           string         `json:"text"`
	Kind           string         `json:"kind"`
	MIME           string         `json:"mime,omitempty"`
	DurationMs     int            `json:"durationMs,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	Reactions      []ChatReaction `json:"reactions"`
}

func EventChatRoom(dateID string) string {
	return EventRoomPrefix + dateID
}

func ParseEventRoom(room string) (string, bool) {
	if !strings.HasPrefix(room, EventRoomPrefix) {
		return "", false
	}
	id := strings.TrimPrefix(room, EventRoomPrefix)
	return id, id != ""
}

func ValidChatRoom(room string) bool {
	return room == RoleChoir || room == RoleBand || room == RoleOrchestra
}

func CanUseChat(role, room string) bool {
	if !ValidChatRoom(room) {
		return false
	}
	if role == RoleChorleiter {
		return true
	}
	return role == room
}

func ChatRoomsForRole(role string) []string {
	if role == RoleChorleiter {
		return []string{RoleChoir, RoleBand, RoleOrchestra}
	}
	if ValidChatRoom(role) {
		return []string{role}
	}
	return nil
}

func ValidChatEmoji(emoji string) bool {
	for _, allowed := range ChatEmojis {
		if emoji == allowed {
			return true
		}
	}
	return false
}

func EventChatAnchor(d Date) time.Time {
	t := d.StartsAt
	for _, o := range d.Options {
		if o.StartsAt.After(t) {
			t = o.StartsAt
		}
	}
	return t
}

func EventChatClosesAt(d Date) time.Time {
	t := EventChatAnchor(d).UTC()
	if t.IsZero() {
		return time.Time{}
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return day.Add(48 * time.Hour)
}

func EventChatIsOpen(d Date, at time.Time) bool {
	closes := EventChatClosesAt(d)
	if closes.IsZero() {
		return false
	}
	return at.UTC().Before(closes)
}

func prepareChatText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("message is required")
	}
	if len([]rune(text)) > 2000 {
		return "", fmt.Errorf("message is too long")
	}
	return text, nil
}

func (s *Store) resolveChatRoom(room string, role string, write bool) error {
	if ValidChatRoom(room) {
		if role != "" && !CanUseChat(role, room) {
			return fmt.Errorf("%w: this chat is not for your role", ErrForbidden)
		}
		return nil
	}
	dateID, ok := ParseEventRoom(room)
	if !ok {
		return fmt.Errorf("unknown chat")
	}
	d, err := s.dateRow(dateID)
	if err != nil {
		return fmt.Errorf("unknown chat")
	}
	if role != "" && !RoleSeesDate(role, d.Roles) {
		return fmt.Errorf("%w: this chat is not for your role", ErrForbidden)
	}
	if write && !EventChatIsOpen(d, now()) {
		return fmt.Errorf("this chat is closed")
	}
	if !EventChatIsOpen(d, now()) {
		return fmt.Errorf("this chat is closed")
	}
	return nil
}

func (s *Store) ChatRoomRoles(room string) []string {
	if ValidChatRoom(room) {
		return []string{room, RoleChorleiter}
	}
	dateID, ok := ParseEventRoom(room)
	if !ok {
		return nil
	}
	d, err := s.dateRow(dateID)
	if err != nil {
		return nil
	}
	return DateAudienceRoles(d.Roles)
}

func (s *Store) AddChatMessage(userID, room, text string) (ChatMessage, error) {
	text, err := prepareChatText(text)
	if err != nil {
		return ChatMessage{}, err
	}
	u, err := s.UserByID(userID)
	if err != nil {
		return ChatMessage{}, err
	}
	if err := s.resolveChatRoom(room, u.Role, true); err != nil {
		return ChatMessage{}, err
	}
	msg := ChatMessage{
		ID:             newID(),
		Room:           room,
		UserID:         u.ID,
		Nickname:       u.Nickname,
		HasPhoto:       u.HasPhoto,
		PhotoUpdatedAt: u.PhotoUpdatedAt,
		Text:           text,
		Kind:           ChatKindText,
		CreatedAt:      now(),
		Reactions:      []ChatReaction{},
	}
	_, err = s.db.Exec(
		`INSERT INTO chat_messages(id, room, user_id, text, created_at, kind) VALUES(?,?,?,?,?,?)`,
		msg.ID, msg.Room, msg.UserID, msg.Text, fmtTime(msg.CreatedAt), msg.Kind,
	)
	if err != nil {
		return ChatMessage{}, err
	}
	return msg, nil
}

func (s *Store) AddAdminChatMessage(room, text string) (ChatMessage, error) {
	text, err := prepareChatText(text)
	if err != nil {
		return ChatMessage{}, err
	}
	if err := s.resolveChatRoom(room, "", true); err != nil {
		return ChatMessage{}, err
	}
	msg := ChatMessage{
		ID:        newID(),
		Room:      room,
		Nickname:  s.AdminAlias(),
		IsAdmin:   true,
		Text:      text,
		Kind:      ChatKindText,
		CreatedAt: now(),
		Reactions: []ChatReaction{},
	}
	_, err = s.db.Exec(
		`INSERT INTO chat_messages(id, room, user_id, text, created_at, kind) VALUES(?,?,NULL,?,?,?)`,
		msg.ID, msg.Room, msg.Text, fmtTime(msg.CreatedAt), msg.Kind,
	)
	if err != nil {
		return ChatMessage{}, err
	}
	return msg, nil
}

func (s *Store) SetChatVoiceText(id, text string) (ChatMessage, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return ChatMessage{}, fmt.Errorf("message is required")
	}
	if runes := []rune(text); len(runes) > 2000 {
		text = string(runes[:2000])
	}
	res, err := s.db.Exec(`UPDATE chat_messages SET text=? WHERE id=? AND kind=?`, text, id, ChatKindVoice)
	if err != nil {
		return ChatMessage{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ChatMessage{}, ErrNotFound
	}
	return s.getChatMessage(id, chatReactionActor, true)
}

func (s *Store) ListChatMessages(role, room, viewerID string) ([]ChatMessage, error) {
	if err := s.resolveChatRoom(room, role, false); err != nil {
		return nil, err
	}
	return s.listChatMessages(room, viewerID, false)
}

func (s *Store) ListChatMessagesForRoom(room string) ([]ChatMessage, error) {
	if err := s.resolveChatRoom(room, "", false); err != nil {
		return nil, err
	}
	return s.listChatMessages(room, chatReactionActor, true)
}

func (s *Store) listChatMessages(room, viewerID string, admin bool) ([]ChatMessage, error) {
	rows, err := s.db.Query(`
SELECT c.id, c.room, c.user_id, u.nickname, c.text, c.created_at, p.updated_at, c.kind, c.mime, c.duration_ms
FROM chat_messages c
LEFT JOIN users u ON u.id = c.user_id
LEFT JOIN user_photos p ON p.user_id = c.user_id
WHERE c.room=?
ORDER BY c.created_at DESC
LIMIT 200`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rev := []ChatMessage{}
	ids := []string{}
	for rows.Next() {
		m, err := scanChatMessage(rows)
		if err != nil {
			return nil, err
		}
		rev = append(rev, m)
		ids = append(ids, m.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]ChatMessage, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	out, err = s.attachReactions(out, ids, viewerID, admin)
	if err != nil {
		return nil, err
	}
	return s.withAdminAlias(out), nil
}

func (s *Store) DeleteChatMessage(userID, room, messageID string, admin bool) error {
	role := ""
	if !admin {
		u, err := s.UserByID(userID)
		if err != nil {
			return err
		}
		role = u.Role
	}
	if err := s.resolveChatRoom(room, role, true); err != nil {
		return err
	}
	var foundRoom string
	var owner sql.NullString
	err := s.db.QueryRow(`SELECT room, user_id FROM chat_messages WHERE id=?`, messageID).Scan(&foundRoom, &owner)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if foundRoom != room {
		return ErrNotFound
	}
	if !admin {
		if !owner.Valid || owner.String != userID {
			return fmt.Errorf("%w: you can only delete your own messages", ErrForbidden)
		}
	}
	_, err = s.db.Exec(`DELETE FROM chat_messages WHERE id=?`, messageID)
	if err != nil {
		return err
	}
	s.removeChatVoiceFile(messageID)
	return nil
}

func (s *Store) ToggleChatReaction(userID, room, messageID, emoji string, admin bool) (ChatMessage, error) {
	if !ValidChatEmoji(emoji) {
		return ChatMessage{}, fmt.Errorf("unknown reaction")
	}
	role := ""
	actor := chatReactionActor
	if !admin {
		u, err := s.UserByID(userID)
		if err != nil {
			return ChatMessage{}, err
		}
		role = u.Role
		actor = u.ID
	}
	if err := s.resolveChatRoom(room, role, true); err != nil {
		return ChatMessage{}, err
	}
	var foundRoom string
	err := s.db.QueryRow(`SELECT room FROM chat_messages WHERE id=?`, messageID).Scan(&foundRoom)
	if err == sql.ErrNoRows {
		return ChatMessage{}, ErrNotFound
	}
	if err != nil {
		return ChatMessage{}, err
	}
	if foundRoom != room {
		return ChatMessage{}, ErrNotFound
	}
	var exists int
	if err := s.db.QueryRow(
		`SELECT COUNT(1) FROM chat_reactions WHERE message_id=? AND user_id=? AND emoji=?`,
		messageID, actor, emoji,
	).Scan(&exists); err != nil {
		return ChatMessage{}, err
	}
	if exists > 0 {
		if _, err := s.db.Exec(`DELETE FROM chat_reactions WHERE message_id=? AND user_id=? AND emoji=?`, messageID, actor, emoji); err != nil {
			return ChatMessage{}, err
		}
	} else {
		if _, err := s.db.Exec(`DELETE FROM chat_reactions WHERE message_id=? AND user_id=?`, messageID, actor); err != nil {
			return ChatMessage{}, err
		}
		if _, err := s.db.Exec(
			`INSERT INTO chat_reactions(message_id, user_id, emoji, created_at) VALUES(?,?,?,?)`,
			messageID, actor, emoji, fmtTime(now()),
		); err != nil {
			return ChatMessage{}, err
		}
	}
	return s.getChatMessage(messageID, actor, admin)
}

func (s *Store) getChatMessage(id, viewerID string, admin bool) (ChatMessage, error) {
	row := s.db.QueryRow(`
SELECT c.id, c.room, c.user_id, u.nickname, c.text, c.created_at, p.updated_at, c.kind, c.mime, c.duration_ms
FROM chat_messages c
LEFT JOIN users u ON u.id = c.user_id
LEFT JOIN user_photos p ON p.user_id = c.user_id
WHERE c.id=?`, id)
	m, err := scanChatMessage(row)
	if err == sql.ErrNoRows {
		return ChatMessage{}, ErrNotFound
	}
	if err != nil {
		return ChatMessage{}, err
	}
	out, err := s.attachReactions([]ChatMessage{m}, []string{m.ID}, viewerID, admin)
	if err != nil {
		return ChatMessage{}, err
	}
	out = s.withAdminAlias(out)
	return out[0], nil
}

func scanChatMessage(rs rowScanner) (ChatMessage, error) {
	var m ChatMessage
	var userID, nickname sql.NullString
	var created string
	var photoAt sql.NullString
	var kind, mime sql.NullString
	var duration sql.NullInt64
	if err := rs.Scan(&m.ID, &m.Room, &userID, &nickname, &m.Text, &created, &photoAt, &kind, &mime, &duration); err != nil {
		return ChatMessage{}, err
	}
	m.CreatedAt = parseTime(created)
	m.Reactions = []ChatReaction{}
	m.Kind = strings.TrimSpace(kind.String)
	if m.Kind == "" {
		m.Kind = ChatKindText
	}
	m.MIME = strings.TrimSpace(mime.String)
	if duration.Valid && duration.Int64 > 0 {
		m.DurationMs = int(duration.Int64)
	}
	if userID.Valid && strings.TrimSpace(userID.String) != "" {
		m.UserID = userID.String
		m.Nickname = nickname.String
	} else {
		m.IsAdmin = true
		m.Nickname = ChatAdminName
	}
	if photoAt.Valid && strings.TrimSpace(photoAt.String) != "" {
		m.HasPhoto = true
		t := parseTime(photoAt.String)
		m.PhotoUpdatedAt = &t
	}
	return m, nil
}

func (s *Store) attachReactions(msgs []ChatMessage, ids []string, viewerID string, admin bool) ([]ChatMessage, error) {
	byID := map[string][]ChatReaction{}
	if len(ids) == 0 {
		return msgs, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`
SELECT message_id, user_id, emoji
FROM chat_reactions
WHERE message_id IN (`+placeholders+`)
ORDER BY created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type key struct {
		messageID string
		emoji     string
	}
	counts := map[key]int{}
	mine := map[key]bool{}
	order := []key{}
	seen := map[key]bool{}
	for rows.Next() {
		var messageID, userID, emoji string
		if err := rows.Scan(&messageID, &userID, &emoji); err != nil {
			return nil, err
		}
		k := key{messageID, emoji}
		if !seen[k] {
			order = append(order, k)
			seen[k] = true
		}
		counts[k]++
		if admin && userID == chatReactionActor {
			mine[k] = true
		}
		if !admin && userID == viewerID {
			mine[k] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, k := range order {
		byID[k.messageID] = append(byID[k.messageID], ChatReaction{
			Emoji: k.emoji,
			Count: counts[k],
			Mine:  mine[k],
		})
	}
	for i := range msgs {
		if reacts := byID[msgs[i].ID]; reacts != nil {
			msgs[i].Reactions = reacts
		}
	}
	return msgs, nil
}

func (s *Store) migrateChatReads() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS chat_reads (
  actor TEXT NOT NULL,
  room TEXT NOT NULL,
  last_seen TEXT NOT NULL,
  PRIMARY KEY (actor, room)
)`)
	return err
}

func (s *Store) MarkChatRead(actor, room, role string) error {
	if err := s.resolveChatRoom(room, role, false); err != nil {
		return err
	}
	_, err := s.db.Exec(`
INSERT INTO chat_reads(actor, room, last_seen) VALUES(?,?,?)
ON CONFLICT(actor, room) DO UPDATE SET last_seen=excluded.last_seen`,
		actor, room, fmtTime(now()))
	return err
}

func (s *Store) MemberChatUnread(u User) (map[string]int, error) {
	return s.chatUnreadCounts(u.ID, ChatRoomsForRole(u.Role), u.ID, false)
}

func (s *Store) ControllerChatUnread() (map[string]int, error) {
	return s.chatUnreadCounts(ChatReadController, []string{RoleChoir, RoleBand, RoleOrchestra}, "", true)
}

func (s *Store) chatUnreadCounts(actor string, rooms []string, excludeUserID string, excludeAdmin bool) (map[string]int, error) {
	out := map[string]int{}
	if len(rooms) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(rooms))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, 2+len(rooms))
	args = append(args, actor)
	for _, room := range rooms {
		args = append(args, room)
	}
	filter := ""
	if excludeAdmin {
		filter = " AND c.user_id IS NOT NULL AND TRIM(c.user_id) != ''"
	} else {
		filter = " AND (c.user_id IS NULL OR c.user_id != ?)"
		args = append(args, excludeUserID)
	}
	rows, err := s.db.Query(`
SELECT c.room, COUNT(*)
FROM chat_messages c
LEFT JOIN chat_reads r ON r.actor=? AND r.room=c.room
WHERE c.room IN (`+placeholders+`)
AND c.created_at > COALESCE(r.last_seen, '')`+filter+`
GROUP BY c.room`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var room string
		var n int
		if err := rows.Scan(&room, &n); err != nil {
			return nil, err
		}
		if n > 0 {
			out[room] = n
		}
	}
	return out, rows.Err()
}
