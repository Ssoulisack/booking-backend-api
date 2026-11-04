# Booking — backend

Go (Fiber) API of the sport booking system, **client edition**: one install = one company with branches.
Owners and admins run the company, managers and staff run a branch (sports, courts, coaches, hours, prices,
classes, desk bookings, cash-up, reports), and customers or guests book on the public pages.

The website is `../booking-system-frontend` (React). The plan and progress of both are in
`../timeline-client-edition/`.

| API group (`/api/v1/…`)                | Who                              | What                                                                  |
| -------------------------------------- | -------------------------------- | --------------------------------------------------------------------- |
| `/auth`                                | Everyone                         | Register, log in (email or phone), Google / Facebook, refresh, forgot / reset password, my account |
| `/public/company`, `/public/branches`  | Everyone, no login               | Company brand, live branches, a branch's sports, courts, coaches, classes, availability, booking   |
| `/me/bookings`                         | Logged-in customers              | My bookings at every branch, cancel                                   |
| `/company`                             | Owner, admins                    | Company settings, branches, team and invitations, customers, accounts, rules, uploads, report |
| `/branches/{branchId}`                 | Manager, staff (+ owner, admins) | One branch: settings, publish, setup, desk bookings, walk-ins, cash-up, reports |

Postman collections of every route are in `postman/` (back office and public).

## Run locally

Needs **Go 1.25+** and Docker.

```sh
cp config.example.yaml config.yaml          # fill in your values; config.yaml holds secrets
docker compose up -d                        # Postgres + Redis; needs a .env with POSTGRES_* and REDIS_PORT
make migrate                                # create the tables (once, and after updates that change them)
make run                                    # http://localhost:3000
```

- The API does **not** change the database on start: `make migrate` creates / updates the tables (it only adds
  what is missing and never deletes data). On an empty database the API's first start creates the company and
  its owner from the `company` block of `config.yaml` (with `ownerPassword` set, you can log in at once).
- After an update that changes the database: back up → new code → `make migrate` → start.
- Emails (invites, password resets, booking mails) go through the SMTP service in the `mailer` block (Resend).
  To catch them locally instead, start Mailpit (`docker compose --profile test up -d mailpit`) and point
  `mailer` at `localhost:1025`; its inbox is http://localhost:8025.
- Settings can also come from environment variables: `APP_` + the key path with `_`, e.g.
  `APP_STORAGE_R2_SECRETKEY`, `APP_DATABASE_POSTGRES_PASSWORD`.

## Configuration (`config.yaml`)

Every setting is explained in `config.example.yaml`. The main blocks:

| Block               | What                                                                                       |
| ------------------- | ------------------------------------------------------------------------------------------ |
| `app`               | Name, `frontendUrl` (links in emails)                                                      |
| `server`            | Port, timeouts, `proxyHeader` / `trustedProxies` behind a proxy or tunnel                  |
| `database.postgres` | Any PostgreSQL: local Docker, own server, or Supabase — only this block changes            |
| `redis`             | Redis address                                                                              |
| `jwt`               | Access and refresh token secrets (32+ characters)                                          |
| `storage`           | Uploaded pictures; `driver` picks one section: `local`, `r2` or `supabase` (see below)     |
| `mailer`            | Any SMTP sender: Mailpit for tests, a sending service (e.g. Resend, Brevo) for real use     |
| `social`            | Google client id, Facebook app id / secret (empty = that login is off)                     |
| `company`           | The company and first owner created on a fresh install                                    |

### File storage

| `storage.driver` | Pictures are kept                         | Links                                                        |
| ---------------- | ----------------------------------------- | ------------------------------------------------------------ |
| `local`          | In `storage.local.path` on this machine; the API serves them at `/files/*` | `storage.local.publicBaseUrl` (empty = `/files/…` on the same domain) |
| `r2`             | Cloudflare R2 (or any S3-compatible service: AWS S3, Backblaze B2, DigitalOcean Spaces) | `storage.r2.publicBaseUrl`, the bucket's custom domain (required) |
| `supabase`       | Supabase Storage                          | Supabase public URL, or `storage.supabase.publicBaseUrl`     |

Only the database keeps the file name (e.g. `branches/3/covers/<uuid>.png`); links are built on every request,
so changing `publicBaseUrl` updates every picture at once. Switching driver means changing config and copying
the files once. With `local`, back up the folder together with the database.

## Commands

| Command                            | Does                                                                                   |
| ---------------------------------- | -------------------------------------------------------------------------------------- |
| `make run`                         | Run the API                                                                            |
| `make migrate`                     | Create / update the tables of the database in `config.yaml`, then stop                 |
| `go test ./test/...`               | Unit tests (`test/domain`, `test/services`, `test/storage`, `test/http`)               |
| `E2E=1 go test ./test/e2e -count=1`| Whole API against a local Postgres + Mailpit (`docker compose --profile test up -d`); needs a database `booking_test` (see `test/e2e/e2e_test.go`) |
| `make db-clear` / `make db-drop`   | Empty / drop every table of the database in `config.yaml` (asks you to type its name)  |
| `make build`                       | Docker image `booking-api:<version>` (version from the `version` file)                 |
| `make release` / `release-dry`     | Multi-arch image (amd64 + arm64) pushed to Docker Hub / built only                     |
| `make run-docker`                  | Run the image with `config.yaml` mounted (never baked into the image)                  |

## Code map

Hexagonal layout: the core knows nothing about HTTP or the database.

```
cmd/
  api/                 main: load config, connect, install, start the server
  migrate/             make migrate: create / update the tables, then stop
internal/
  app/                 wiring: repositories → services → handlers → routes (+ /files when local storage)
  platform/            shared plumbing
    config/            config.yaml + APP_* environment variables (viper)
    postgres/          connection, GORM repositories, migration/ (run by cmd/migrate), numbering, errors
    redis/  mailer/    cache · SMTP mails
    storage/           storage.go (config, pick driver) · local.go · s3.go (R2) · supabase.go
    social/            Google / Facebook token checks
    middleware/        auth, rate limits
  core/                business code (moving to modules)
    domain/            models and rules (bookings, availability, roles, identity)
    services/          business logic per area (company, branch, catalog, schedule, booking, desk, reports…)
    port/              input (service) and output (repository, storage, mailer) interfaces
    dto/, mappers/     API shapes
  http/                Fiber server, handlers, routes
pkg/                   shared helpers (responses, errors, JWT, rate limiter, logger, utils)
test/                  Go tests by area; e2e/ runs the whole API
postman/               Postman collections
scripts/               release.sh, reset-db.sh
```
