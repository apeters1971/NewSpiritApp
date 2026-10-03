package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/mail"
	"github.com/apeters/newspirit/internal/store"
)

type publicVoteOption struct {
	ID       string     `json:"id"`
	StartsAt time.Time  `json:"startsAt"`
	EndsAt   *time.Time `json:"endsAt,omitempty"`
	MyChoice string     `json:"myChoice"`
}

type publicVoteView struct {
	Nickname         string             `json:"nickname"`
	Title            string             `json:"title"`
	Category         string             `json:"category"`
	StartsAt         time.Time          `json:"startsAt"`
	EndsAt           *time.Time         `json:"endsAt,omitempty"`
	Location         string             `json:"location,omitempty"`
	Status           string             `json:"status"`
	PollOpen         bool               `json:"pollOpen"`
	MyChoice         string             `json:"myChoice"`
	Options          []publicVoteOption `json:"options,omitempty"`
	RemainingSeconds int                `json:"remainingSeconds"`
	LocationOwner    bool               `json:"locationOwner"`
}

func (s *Server) mailFrom() string {
	if s.Store != nil {
		if from := s.Store.MailFrom(); from != "" {
			return from
		}
	}
	if s.MailFrom != "" {
		return s.MailFrom
	}
	return mail.DefaultFrom
}

func (s *Server) mailer() mail.Sender {
	from := s.mailFrom()
	if box, ok := s.Mailer.(*mail.Memory); ok {
		box.From = from
		return box
	}
	if s.Mailer != nil {
		return s.Mailer
	}
	return mail.Sendmail{From: from, Domain: s.MailDomain}
}

func (s *Server) handleControllerVoteRemind(w http.ResponseWriter, r *http.Request) {
	if !s.requireController(w, r) {
		return
	}
	s.sendVoteReminders(w, r, r.PathValue("id"), nil)
}

func (s *Server) handleMemberVoteRemind(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := s.Store.CanSendVoteReminder(user, r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	s.sendVoteReminders(w, r, r.PathValue("id"), &user)
}

func (s *Server) sendVoteReminders(w http.ResponseWriter, r *http.Request, dateID string, actor *store.User) {
	d, err := s.Store.DateView(dateID, actor)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !store.DateIsCurrent(d.Date) {
		writeError(w, http.StatusBadRequest, "voting is closed")
		return
	}
	links, err := s.Store.ReplaceVoteReminderLinks(dateID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	base := requestBaseURL(r)
	sent, failed := 0, 0
	for _, link := range links {
		when := store.FormatChoirWhen(d.StartsAt)
		if d.EndsAt != nil {
			end := store.FormatChoirWhen(*d.EndsAt)
			if end != "" && end != when {
				when = when + " – " + end
			}
		}
		body := voteReminderBody(link.Nickname, d.Title, when, d.Location, base+"/vote/"+link.Token)
		if err := s.mailer().Send(link.Email, voteReminderSubject(d.Title), body); err != nil {
			failed++
			continue
		}
		sent++
	}
	if sent == 0 {
		writeError(w, http.StatusBadGateway, "mail failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": sent, "failed": failed})
}

func voteReminderSubject(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New Spirit"
	}
	return "New Spirit: " + title + " — bitte abstimmen"
}

func voteReminderBody(nickname, title, when, location, url string) string {
	name := strings.TrimSpace(nickname)
	if name == "" {
		name = "du"
	}
	lines := []string{
		"Hallo " + name + ",",
		"",
		"bitte stimme für diesen Termin ab:",
		"",
		strings.TrimSpace(title),
	}
	if when != "" {
		lines = append(lines, when)
	}
	if loc := strings.TrimSpace(location); loc != "" {
		lines = append(lines, loc)
	}
	lines = append(lines,
		"",
		url,
		"",
		"Der Link gilt eine Woche. Nach dem ersten Öffnen bleiben fünf Minuten.",
		"",
		"—",
		"",
		"Hello "+name+",",
		"",
		"please vote for this date:",
		"",
		strings.TrimSpace(title),
	)
	if when != "" {
		lines = append(lines, when)
	}
	if loc := strings.TrimSpace(location); loc != "" {
		lines = append(lines, loc)
	}
	lines = append(lines,
		"",
		url,
		"",
		"The link lasts one week. After you open it the first time, five minutes remain.",
	)
	return strings.Join(lines, "\n")
}

func (s *Server) handleVotePage(w http.ResponseWriter, r *http.Request) {
	serveFSFile(w, r, s.ClientFS, "vote.html")
}

func (s *Server) handleVoteLink(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sess, err := s.Store.OpenVoteReminder(token)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.publicVoteView(sess)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleVoteLinkCast(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sess, err := s.Store.VoteReminderSession(token)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !store.DateIsCurrent(sess.Date) {
		writeError(w, http.StatusBadRequest, "voting is closed")
		return
	}
	var body struct {
		Choice   string `json:"choice"`
		OptionID string `json:"optionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if sess.Date.PollOpen && !store.OwnsVenue(sess.User, sess.Date) {
		err = s.Store.SetPollVote(sess.User.ID, sess.Date.ID, body.OptionID, body.Choice)
	} else {
		err = s.Store.SetVote(sess.User.ID, sess.Date.ID, body.Choice)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	s.Hub.Broadcast(hub.Envelope{Type: "changed"})
	fresh, err := s.Store.VoteReminderSession(token)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	view, err := s.publicVoteView(fresh)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) publicVoteView(sess store.VoteReminderSession) (publicVoteView, error) {
	view, err := s.Store.DateView(sess.Date.ID, &sess.User)
	if err != nil {
		return publicVoteView{}, err
	}
	out := publicVoteView{
		Nickname:         sess.User.Nickname,
		Title:            view.Title,
		Category:         view.Category,
		StartsAt:         view.StartsAt,
		EndsAt:           view.EndsAt,
		Location:         view.Location,
		Status:           view.Status,
		PollOpen:         view.PollOpen,
		MyChoice:         view.MyChoice,
		RemainingSeconds: int(sess.Remaining / time.Second),
		LocationOwner:    store.OwnsVenue(sess.User, view.Date),
	}
	if view.PollOpen && !out.LocationOwner {
		out.Options = make([]publicVoteOption, 0, len(view.Options))
		for _, o := range view.Options {
			out.Options = append(out.Options, publicVoteOption{
				ID:       o.ID,
				StartsAt: o.StartsAt,
				EndsAt:   o.EndsAt,
				MyChoice: o.MyChoice,
			})
		}
	}
	return out, nil
}
