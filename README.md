# New Spirit

Availability voting for choir, band, orchestra, and technicians.

Members sign in with the email and password you create in the controller. Each date is visible to the roles you attach. Everyone who can see a date sees the full roster, every vote, and counts by subrole.

## Quick start

```bash
export CONTROLLER_SECRET=dev-secret
make run
```

- Member UI: [http://localhost:8080/](http://localhost:8080/)
- Controller UI: [http://localhost:8080/controller](http://localhost:8080/controller)

HTTPS on port 8443 (certificate + private key):

```bash
go run ./cmd/server -cert /path/to/cert.pem -key /path/to/key.pem
# or: make run TLS_CERT=/path/to/cert.pem TLS_KEY=/path/to/key.pem
```

Create people and dates in the controller first. Members cannot self-register.

## Environment

| Variable | Default | Meaning |
|----------|---------|---------|
| `ADDR` | `:8080` (`:8443` with TLS) | Listen address |
| `CONTROLLER_SECRET` | _(empty)_ | Shared secret for `/controller` |
| `DATA_DIR` | `data` | SQLite directory |
| `TLS_CERT` | _(empty)_ | Certificate PEM; enables HTTPS |
| `TLS_KEY` | _(empty)_ | Private key PEM (or a sibling of `TLS_CERT`) |

## Date lifecycle

Dates start in **Voting**. The controller can finalize them as **Accepted** or **Cancelled**.

- Voting: members can change their vote freely.
- Accepted: members can still change their vote. The vote at accept time stays visible as the initial vote.
- Cancelled: the date stays visible; voting is locked.
