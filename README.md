# New Spirit

Availability, chat, and day-of-show tools for **New Spirit** — choir, choir director, band, orchestra, technicians, and alumni.

One Go server, SQLite, and a German/English web UI. Members use the phone-friendly site. The controller (Verwaltung) creates people and dates. Nobody self-registers.

| | |
| --- | --- |
| Members | [http://localhost:8080/](http://localhost:8080/) |
| Controller | [http://localhost:8080/controller](http://localhost:8080/controller) |

On a phone, **Add App** adds the member UI like an app (Safari: Share → Add to Home Screen; Android: Install app). That works best over HTTPS.

## Quick start

You need [Go](https://go.dev/dl/) 1.26+.

```bash
export CONTROLLER_SECRET=dev-secret
make run
```

1. Open the controller and sign in with `CONTROLLER_SECRET`.
2. Add people (email + one-time password + role).
3. Add dates and attach the roles that should see them.
4. Members sign in with that email and password, then set their own password on first login.

HTTPS (needed for a real calendar feed and a reliable home-screen install):

```bash
make run TLS_CERT=/path/to/cert.pem TLS_KEY=/path/to/key.pem
```

That listens on `:8443` unless you set `ADDR`.

```bash
go test ./...
```

## What members get

- Dates for their role, with votes (**Ja / Vielleicht / Nein**) and a full roster
- Time polls when a date offers several slots
- Next accepted date they said yes or maybe to, plus an upcoming list
- Comments, title lists from the music archive, and an event chat while the date is open
- Role chats (choir, band, orchestra) with reactions
- Own channels (1–96) and 48V, when they sing or play
- Technicians assign mixer channels 1–96 in the member view
- Song proposals, address book (email + phone), personal info, and a profile photo
- Spirit of the Year (choir, choir director, and alumni; names and attendance, no scores)
- Calendar subscribe link for accepted dates of their role
- Live stream: streamers start it with Record in the header; everyone can watch with Play (computer or phone)
- German / English

**Ehemalige** see choir dates and the choir chat so they can follow along. They cannot vote. They can have mixer channels.

## What the controller gets

Tabs: **Personen**, **Kontakte**, **Termine**, **Notenarchiv**, **Kanäle**, **Songvorschläge**, **Rang**, **Einstellungen**.

- Create and edit people, one-time passwords, photos, contact info, and the Streamer checkbox
- Create dates: category, location, notes, schedule, what to bring (mic, cable, stand, white/black cloth, casual), roles, and optional time polls
- Accept or cancel a date; freeze a poll to the chosen time
- Music archive (audio, lyrics, sheets) and attach titles to dates
- Assign mixer channels 1–96
- Accept, decline, or comment on song proposals
- Choir ranking (Spirit of the Year; ties share the lead)

## Roles

| Role | Typical parts |
| --- | --- |
| Chor | Sopran, Alt, Tenor/Bass |
| Chorleiter | Chorleiter |
| Band | Drums, Percussion, Guitar, Hammond, E-Bass, Trumpet, Sax, Trombone, Piano |
| Orchester | Strings, Woodbrass, Brass, Percussion, Harp |
| Technik | Sound, Light, Stage |
| Ehemalige | Ehemalige |

A date is visible to the roles you check. Choir director and alumni also see choir dates. Alumni cannot vote.

## Dates

Dates start in **Abstimmung** (voting). Finalize them as **Angenommen** or **Abgesagt**.

- **Voting** — members can change their vote. Open polls stay open until you freeze a time.
- **Accepted** — still votable. The vote at accept time is kept as the first vote. **Nächster Termin** only uses accepted dates the member marked Ja or Vielleicht.
- **Cancelled** — stays visible; voting is locked.

Yes = 2, Maybe = 1, No = 0 for the choir ranking. Switching from Yes to No later scores −1. Open polls and cancelled dates do not count.

## Environment

| Variable | Default | Meaning |
| --- | --- | --- |
| `CONTROLLER_SECRET` | _(empty)_ | Shared secret for `/controller`. Required in production. |
| `ADDR` | `:8080` (`:8443` with TLS) | Listen address |
| `DATA_DIR` | `data` | SQLite directory (`data/spirit.db`) |
| `TLS_CERT` | _(empty)_ | Certificate PEM; enables HTTPS |
| `TLS_KEY` | _(empty)_ | Private key PEM (or a sibling of `TLS_CERT`) |

`data/` is gitignored. Cookies are `spirit_session` (members) and `spirit_controller` (admin).

## Layout

```
cmd/server/          HTTP server
internal/api/        Routes and sessions
internal/store/      SQLite
internal/hub/        Live updates (WebSocket)
web/client/          Member UI (embedded)
web/controller/      Controller UI (embedded)
```

The UI is compiled into the binary (`go:embed`). After you change `web/`, restart the server.

## Build

```bash
make build    # bin/spirit
make tidy
```
