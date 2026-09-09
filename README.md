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

Create people and dates in the controller first. Members cannot self-register.

## Environment

| Variable | Default | Meaning |
|----------|---------|---------|
| `ADDR` | `:8080` | HTTP listen address |
| `CONTROLLER_SECRET` | _(empty)_ | Shared secret for `/controller` |
| `DATA_DIR` | `data` | SQLite directory |

## Date lifecycle

Dates start in **Voting**. The controller can finalize them as **Accepted** or **Cancelled**.

- Voting: members can change their vote freely.
- Accepted: members can still change their vote. The vote at accept time stays visible as the initial vote.
- Cancelled: the date stays visible; voting is locked.
