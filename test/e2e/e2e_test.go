// Package e2e runs the whole API in-process against a real local Postgres and Mailpit.
//
//	docker compose up -d postgres redis mailpit
//	docker exec postgres_db psql -U booking -c "CREATE DATABASE booking_test"   # once
//	E2E=1 go test ./test/e2e -count=1 -v
//
// The test database is wiped at the start, so the test refuses any database that is not
// local and whose name does not end in "_test".
package e2e

import (
	"backend/cmd/application"
	"backend/internal/adapters/secondary/mailer"
	"backend/internal/adapters/secondary/minio"
	"backend/internal/adapters/secondary/postgres"
	"backend/internal/adapters/secondary/redis"
	"backend/internal/adapters/secondary/storage"
	"backend/internal/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var mailpitAPI = env("E2E_MAILPIT_API", "http://localhost:8025/api/v1")

// ---------- setup ----------

func newApp(t *testing.T) *fiber.App {
	t.Helper()
	return newAppWith(t, ownerPassword)
}

// newAppWith starts a fresh install; an empty password makes the install email the owner a link
func newAppWith(t *testing.T, password string) *fiber.App {
	t.Helper()
	if os.Getenv("E2E") == "" {
		t.Skip("set E2E=1 to run (needs local Postgres, Redis and Mailpit)")
	}
	pg := postgres.Config{
		Host: env("E2E_DB_HOST", "localhost"), Port: env("E2E_DB_PORT", "5432"),
		User: env("E2E_DB_USER", "booking"), Password: env("E2E_DB_PASSWORD", "booking"),
		DBName: env("E2E_DB_NAME", "booking_test"), SSLMode: "disable", TimeZone: "Asia/Vientiane",
	}
	if (pg.Host != "localhost" && pg.Host != "127.0.0.1") || !strings.HasSuffix(pg.DBName, "_test") {
		t.Fatalf("refusing to wipe %s/%s: only a local *_test database is allowed", pg.Host, pg.DBName)
	}
	resetDatabase(t, pg)
	resetMailpit(t)

	files := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(files.Close)

	config.JWTGlobal = config.JWTConfig{Secret: "e2e-access-secret-0123456789abcd", RefreshSecret: "e2e-refresh-secret-0123456789abc"}
	cfg := &config.Config{
		App:      config.AppConfig{Name: "Sport Booking E2E", FrontendURL: "http://frontend.test"},
		Postgres: pg,
		Redis:    redis.Config{Host: env("E2E_REDIS_HOST", "localhost"), Port: "6379"},
		Mailer:   mailer.Config{Host: env("E2E_SMTP_HOST", "localhost"), Port: 1025, From: "no-reply@booking.test"},
		Minio:    minio.Config{Endpoint: "localhost:9000"}, // template adapter exits on an empty endpoint; unused
		Storage:  storage.Config{ProjectURL: files.URL, Bucket: "media", ServiceKey: "fake"},
		Company:  config.CompanyConfig{Name: "E2E Sports Co", OwnerName: "Olga Owner", OwnerEmail: ownerEmail, OwnerPassword: password},
	}
	repos := application.SetupRepositories(cfg)
	t.Cleanup(func() { application.CloseRepositories(repos) })

	services := application.SetupServices(repos)
	if err := application.Install(services); err != nil {
		t.Fatalf("install: %v", err)
	}
	app := fiber.New()
	application.SetupRoutes(app, application.SetupHandlers(services))
	return app
}

// the company owner the test install creates (config.yaml company block)
const ownerEmail, ownerPassword = "owner@company.test", "password123"

// ownerToken logs the company owner in
func ownerToken(a api) string {
	a.t.Helper()
	return a.expect(200, "", "POST", "/auth/login", "", map[string]any{"login": ownerEmail, "password": ownerPassword}).str("data.access_token")
}

// teamMember invites someone and accepts as a new account: role "admin" or "manager" / "staff"
// at the branches given; returns their access token
func teamMember(a api, owner, email, role string, branchIDs ...string) string {
	a.t.Helper()
	ids := []int{}
	for _, id := range branchIDs {
		ids = append(ids, num(id))
	}
	a.expect(201, "", "POST", "/company/invitations", owner, map[string]any{"email": email, "role": role, "branch_ids": ids})
	return a.expect(200, "", "POST", "/invitations/"+linkFor(a.t, email, "/invite/")+"/accept", "",
		map[string]any{"name": strings.Split(email, "@")[0], "password": "password123"}).str("data.access_token")
}

func resetDatabase(t *testing.T, pg postgres.Config) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		pg.Host, pg.Port, pg.User, pg.Password, pg.DBName)
	db, err := gorm.Open(gormpg.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;").Error; err != nil {
		t.Fatalf("reset test database: %v", err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
}

// ---------- HTTP client ----------

type api struct {
	t   *testing.T
	app *fiber.App
}

type reply struct {
	status int
	body   map[string]any
}

func (r reply) msg() string { s, _ := r.body["message"].(string); return s }

// str reads a dotted path from the body, e.g. "data.user.email"; a number picks a list item ("data.0.id")
func (r reply) str(path string) string {
	var cur any = r.body
	for _, key := range strings.Split(path, ".") {
		switch c := cur.(type) {
		case map[string]any:
			cur = c[key]
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(c) {
				return ""
			}
			cur = c[i]
		default:
			return ""
		}
	}
	switch v := cur.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case nil:
		return ""
	}
	return fmt.Sprint(cur)
}

func (a api) send(req *http.Request, token string) reply {
	a.t.Helper()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.app.Test(req, -1) // no timeout: bcrypt is deliberately slow
	if err != nil {
		a.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := reply{status: res.StatusCode, body: map[string]any{}}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func (a api) call(method, path, token string, body any) reply {
	a.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, rd)
	req.Header.Set("Content-Type", "application/json")
	return a.send(req, token)
}

// expect fails the test unless the call returns want (and wantMsg, when given)
func (a api) expect(want int, wantMsg string, method, path, token string, body any) reply {
	a.t.Helper()
	r := a.call(method, path, token, body)
	if r.status != want || (wantMsg != "" && r.msg() != wantMsg) {
		a.t.Fatalf("%s %s: got %d %q, want %d %q (body %v)", method, path, r.status, r.msg(), want, wantMsg, r.body)
	}
	return r
}

func (a api) upload(path, token, folder string, data []byte) reply {
	a.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "photo.png")
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path+"?folder="+folder, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return a.send(req, token)
}

// ---------- Mailpit ----------

func resetMailpit(t *testing.T) {
	req, _ := http.NewRequest(http.MethodDelete, mailpitAPI+"/messages", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Mailpit not reachable at %s: %v", mailpitAPI, err)
	}
	res.Body.Close()
}

// linkFor returns the token from the newest email to `to` whose link contains pathPart
func linkFor(t *testing.T, to, pathPart string) string {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(pathPart) + `([A-Za-z0-9_-]+)`)
	for range 20 { // the email is sent synchronously, but give Mailpit a moment to index it
		var list struct {
			Messages []struct{ ID string } `json:"messages"`
		}
		getJSON(t, mailpitAPI+"/search?query=to:"+to, &list)
		for _, m := range list.Messages {
			var msg struct{ Text string }
			getJSON(t, mailpitAPI+"/message/"+m.ID, &msg)
			if found := re.FindStringSubmatch(msg.Text); found != nil {
				return found[1]
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no email to %s with a %s link", to, pathPart)
	return ""
}

func getJSON(t *testing.T, url string, out any) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)

// ---------- the scenario ----------


// num turns an id read from a reply back into a JSON number for request bodies
func num(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic("not a number: " + s)
	}
	return n
}

// raw returns the response body as text (for CSV downloads)
func (a api) raw(method, path, token string) string {
	a.t.Helper()
	req := httptest.NewRequest(method, "/api/v1"+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.app.Test(req, -1)
	if err != nil {
		a.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return string(b)
}

// expectEmail fails unless an email to `to` contains every one of the given texts
func expectEmail(t *testing.T, to string, texts ...string) {
	t.Helper()
	for range 20 {
		var list struct {
			Messages []struct{ ID string } `json:"messages"`
		}
		getJSON(t, mailpitAPI+"/search?query=to:"+to, &list)
		for _, m := range list.Messages {
			var msg struct{ Text string }
			getJSON(t, mailpitAPI+"/message/"+m.ID, &msg)
			all := true
			for _, want := range texts {
				all = all && strings.Contains(msg.Text, want)
			}
			if all {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no email to %s containing %q", to, texts)
}
