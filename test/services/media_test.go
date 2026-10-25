package servicetest

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"backend/internal/core/port/input"
	"backend/internal/core/port/output"
	"backend/internal/core/services"
	"bytes"
	"context"
	"regexp"
	"testing"
)

const (
	branch   = 7
	mb       = 1 << 20
	maxBytes = 2 * mb // the upload limit
)

// keys the server generates for branch 7
var ownKeyPattern = regexp.MustCompile(`^branches/7/sports/[0-9a-f-]{36}\.(png|jpg|webp)$`)

func newMediaService(store *fakeStorage, media *fakeMedia) input.MediaService {
	return services.NewMediaService(store, media)
}

// smallest valid headers; content type is detected from the leading bytes
var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	jpegBytes = append([]byte("\xff\xd8\xff\xe0"), make([]byte, 32)...)
	webpBytes = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
)

func TestMediaUploadAccepts(t *testing.T) {
	for name, data := range map[string][]byte{"png": pngBytes, "jpeg": jpegBytes, "webp": webpBytes} {
		t.Run(name, func(t *testing.T) {
			store, media := newFakeStorage(), newFakeMedia()
			res, err := newMediaService(store, media).Upload(context.Background(), branch, "sports", bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if !ownKeyPattern.MatchString(res.Key) {
				t.Fatalf("bad key %q", res.Key)
			}
			if _, ok := store.uploaded[res.Key]; !ok || res.URL != "https://files.test/"+res.Key {
				t.Fatalf("not stored / wrong url: %+v", res)
			}
			if rec, _ := media.Get(context.Background(), branch, res.Key); rec == nil || rec.SizeBytes != int64(len(data)) {
				t.Fatalf("media record missing or wrong size: %+v", rec)
			}
		})
	}
}

func TestMediaUploadRejects(t *testing.T) {
	tooBig := append(append([]byte{}, pngBytes...), make([]byte, maxBytes)...)
	cases := map[string]struct {
		folder string
		data   []byte
		code   string
	}{
		"executable renamed to jpg": {"sports", []byte("MZ\x90\x00\x03\x00\x00\x00 not an image"), "UNSUPPORTED_FILE_TYPE"},
		"svg (script risk)":         {"sports", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), "UNSUPPORTED_FILE_TYPE"},
		"html":                      {"sports", []byte("<html><body>hi</body></html>"), "UNSUPPORTED_FILE_TYPE"},
		"empty":                     {"sports", nil, "FILE_IS_EMPTY"},
		"too large":                 {"sports", tooBig, "FILE_TOO_LARGE"},
		"unknown folder":            {"../etc", pngBytes, "INVALID_FOLDER"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store := newFakeStorage()
			_, err := newMediaService(store, newFakeMedia()).Upload(context.Background(), branch, tc.folder, bytes.NewReader(tc.data))
			if err == nil || err.Error() != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
			if len(store.uploaded) != 0 {
				t.Fatal("rejected file must not reach storage")
			}
		})
	}
}

// Company files (logo, cover) are stored under branch 0 and belong to the company only
func TestCompanyUpload(t *testing.T) {
	media := newFakeMedia()
	res, err := newMediaService(newFakeStorage(), media).Upload(context.Background(), 0, "branding", bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^branches/0/branding/[0-9a-f-]{36}\.png$`).MatchString(res.Key) || media.items[res.Key].BranchID != 0 {
		t.Fatalf("got key %s, record %+v", res.Key, media.items[res.Key])
	}
}

func TestMediaDeleteOnlyOwnKeys(t *testing.T) {
	ctx := context.Background()
	store, media := newFakeStorage(), newFakeMedia()
	svc := newMediaService(store, media)
	own := "branches/7/coaches/11111111-1111-1111-1111-111111111111.jpg"
	other := "branches/8/coaches/11111111-1111-1111-1111-111111111111.jpg"
	media.items[own] = domain.Media{BranchID: 7, Key: own}
	media.items[other] = domain.Media{BranchID: 8, Key: other}

	for _, bad := range []string{"", "../secret.png", "branches/7/../x.png", "branches/7/other/11111111-1111-1111-1111-111111111111.png", "branches/7/sports/not-a-uuid.png", other} {
		if err := svc.Delete(ctx, branch, bad); err == nil {
			t.Fatalf("key %q must be rejected", bad)
		}
	}
	if len(store.deleted) != 0 {
		t.Fatal("nothing should have been deleted from storage yet")
	}
	if err := svc.Delete(ctx, branch, own); err != nil || len(store.deleted) != 1 {
		t.Fatalf("own key rejected: %v", err)
	}
	if _, ok := media.items[other]; !ok {
		t.Fatal("another branch's file must survive")
	}
}

// A sport may only point at an image this branch uploaded (checked through the catalog service)
func TestSportImageMustBeOwnUpload(t *testing.T) {
	ctx := context.Background()
	media := newFakeMedia()
	sports := &fakeSports{owner: map[uint]uint{1: branch}}
	catalog := services.NewCatalogService(sports, nil, nil, nil, nil, newFakeStorage(), media)

	own := "branches/7/sports/22222222-2222-2222-2222-222222222222.png"
	media.items[own] = domain.Media{BranchID: branch, Key: own}
	notUploaded := "branches/7/sports/33333333-3333-3333-3333-333333333333.png"
	otherBiz := "branches/8/sports/22222222-2222-2222-2222-222222222222.png"

	for _, key := range []string{"", own} {
		if _, err := catalog.UpdateSport(ctx, branch, 1, dto.SportRequest{Name: "Tennis", ImageKey: key}); err != nil {
			t.Fatalf("key %q rejected: %v", key, err)
		}
	}
	for _, bad := range []string{notUploaded, otherBiz, "https://evil.example/x.png"} {
		_, err := catalog.UpdateSport(ctx, branch, 1, dto.SportRequest{Name: "Tennis", ImageKey: bad})
		if err == nil || err.Error() != "INVALID_FILE_KEY" {
			t.Fatalf("key %q: got %v, want INVALID_FILE_KEY", bad, err)
		}
	}
	if len(sports.updated) != 2 {
		t.Fatalf("only the two valid updates may be saved, got %d", len(sports.updated))
	}
}

// The library lists one scope only (a branch never sees the company's files) and says what uses each file
func TestMediaListIsPerScope(t *testing.T) {
	ctx := context.Background()
	media := newFakeMedia()
	svc := newMediaService(newFakeStorage(), media)
	cover := "branches/7/branding/11111111-1111-1111-1111-111111111111.png"
	spare := "branches/7/sports/22222222-2222-2222-2222-222222222222.png"
	logo := "branches/0/branding/33333333-3333-3333-3333-333333333333.png"
	other := "branches/8/sports/44444444-4444-4444-4444-444444444444.png"
	for _, k := range []struct {
		key string
		b   uint
	}{{cover, 7}, {spare, 7}, {logo, 0}, {other, 8}} {
		media.items[k.key] = domain.Media{BranchID: k.b, Key: k.key, SizeBytes: 5}
	}
	media.uses = []output.MediaUse{{Key: cover, Kind: "cover", Name: "Main"}}

	got, err := svc.List(ctx, branch)
	if err != nil || len(got) != 2 {
		t.Fatalf("branch 7 must see only its two files, got %+v (%v)", got, err)
	}
	for _, f := range got {
		if f.Key == cover && (len(f.UsedBy) != 1 || f.UsedBy[0].Kind != "cover") {
			t.Fatalf("cover must say where it is used: %+v", f)
		}
		if f.Key == spare && (f.UsedBy == nil || len(f.UsedBy) != 0) {
			t.Fatalf("an unused file lists an empty used_by, not null: %+v", f)
		}
	}
	if company, _ := svc.List(ctx, 0); len(company) != 1 || company[0].Key != logo {
		t.Fatalf("the company library holds only company files: %+v", company)
	}
}

// A picture still shown somewhere cannot be deleted; an unused one can
func TestMediaDeleteRefusesFileInUse(t *testing.T) {
	ctx := context.Background()
	store, media := newFakeStorage(), newFakeMedia()
	svc := newMediaService(store, media)
	used := "branches/7/sports/11111111-1111-1111-1111-111111111111.png"
	free := "branches/7/sports/22222222-2222-2222-2222-222222222222.png"
	media.items[used] = domain.Media{BranchID: 7, Key: used}
	media.items[free] = domain.Media{BranchID: 7, Key: free}
	media.uses = []output.MediaUse{{Key: used, Kind: "sport", Name: "Tennis"}}

	if err := svc.Delete(ctx, branch, used); err == nil || err.Error() != "FILE_IN_USE" {
		t.Fatalf("got %v, want FILE_IN_USE", err)
	}
	if len(store.deleted) != 0 {
		t.Fatal("a file in use must stay in storage")
	}
	if err := svc.Delete(ctx, branch, free); err != nil || len(store.deleted) != 1 {
		t.Fatalf("unused file not deleted: %v", err)
	}
}
