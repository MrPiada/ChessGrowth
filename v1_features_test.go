package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockTransportV1 func(*http.Request) (*http.Response, error)

func (f mockTransportV1) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fakeResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestSavedLichessTokenPerUserAndDeletion(t *testing.T) {
	store := fileTokenStore{path: filepath.Join(t.TempDir(), "ChessGrowth", "lichess-token")}
	value := "lip_demo_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if err := store.Save(value); err != nil {
		t.Fatal(err)
	}
	again := fileTokenStore{path: store.path}
	loaded, err := again.Load()
	if err != nil || loaded != value {
		t.Fatalf("token did not persist: %v", err)
	}
	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("token file permission %o; need 0600", info.Mode().Perm())
	}
	if err := again.Delete(); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.Load()
	if err != nil || loaded != "" {
		t.Fatal("token was not removed")
	}
	if err := store.Save("bad token containing spaces"); err == nil {
		t.Fatal("invalid token was accepted")
	}
	if err := store.Save(strings.Repeat("a", 2049)); err == nil {
		t.Fatal("oversized token was accepted")
	}
}

func TestCredentialAPIStatusSaveAndOriginProtection(t *testing.T) {
	t.Setenv("LICHESS_TOKEN", "")
	a := newApplication()
	a.tokens = fileTokenStore{path: filepath.Join(t.TempDir(), "token")}
	handler := a.handlers()
	value := "lip_unitTest_ABC123"
	request := func(method, url, body, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, url, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	w := request("POST", "http://127.0.0.1:8766/api/credentials", `{"token":"`+value+`"}`, "https://untrusted.example")
	if w.Code != 403 {
		t.Fatalf("cross-origin token write accepted: %d", w.Code)
	}
	w = request("POST", "http://127.0.0.1:8766/api/credentials", `{"token":"`+value+`"}`, "http://127.0.0.1:8766")
	if w.Code != 200 {
		t.Fatalf("save failed: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), value) {
		t.Fatal("save endpoint leaked the token")
	}
	w = request("GET", "http://127.0.0.1:8766/api/credentials", "", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), value) {
		t.Fatal("GET endpoint failure or leaked token")
	}
	var result struct {
		Saved bool `json:"saved"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.Saved {
		t.Fatalf("not marked saved: %s %v", w.Body.String(), err)
	}
	w = request("DELETE", "http://127.0.0.1:8766/api/credentials", "", "http://127.0.0.1:8766")
	if w.Code != 200 {
		t.Fatalf("delete failed: %d", w.Code)
	}
}

func TestCloudFallbackToStockfishAndDoNotLeakToken(t *testing.T) {
	c := defaults()
	c.EngineMode = "hybrid"
	c.DelayMS = 300
	g := newGenerator(c, nil)
	g.cache = t.TempDir()
	g.token = "lip_privateOnly123"
	invoked := 0
	g.stockfishFn = func(_ context.Context, path, fen string, depth int) (int, error) { invoked++; return 43, nil }
	cloudMode := "missing"
	explorerAuth := "unset"
	g.client = &http.Client{Transport: mockTransportV1(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "explorer.lichess.org" {
			explorerAuth = r.Header.Get("Authorization")
			return fakeResponse(200, `{"white":15,"draws":3,"black":2,"moves":[]}`), nil
		}
		if r.URL.Hostname() != "lichess.org" {
			t.Errorf("unexpected host %s", r.URL.Host)
			return fakeResponse(404, ""), nil
		}
		if r.Header.Get("Authorization") != "Bearer lip_privateOnly123" {
			t.Errorf("cloud missing authentication")
		}
		if cloudMode == "missing" {
			return fakeResponse(404, ""), nil
		}
		return fakeResponse(200, `{"pvs":[{"cp":27}]}`), nil
	})}
	_, err := g.explorer(context.Background(), initialFEN)
	if err != nil {
		t.Fatal(err)
	}
	if explorerAuth != "" {
		t.Fatalf("token leaked to Opening Explorer: %q", explorerAuth)
	}
	e, err := g.eval(context.Background(), initialFEN)
	if err != nil {
		t.Fatal(err)
	}
	if invoked != 1 || e.cp == nil || *e.cp != 43 || e.src != "Stockfish (local)" {
		t.Fatalf("fallback failed: %+v invoked %d", e, invoked)
	}
	cloudMode = "present"
	e, err = g.eval(context.Background(), "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if invoked != 1 || e.cp == nil || *e.cp != 27 || e.src != "Lichess Cloud" {
		t.Fatalf("cloud score ignored: %+v invoked %d", e, invoked)
	}
}

func TestCloudNegativeCacheExpires(t *testing.T) {
	c := defaults()
	g := newGenerator(c, nil)
	g.cache = t.TempDir()
	g.client = &http.Client{Transport: mockTransportV1(func(r *http.Request) (*http.Response, error) { return fakeResponse(404, ""), nil })}
	query := url.Values{"fen": {initialFEN}}
	_, err := g.get(context.Background(), "cloud", "https://lichess.org/api/cloud-eval", query)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(g.cache, "cloud_*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("cache missing %v %v", matches, err)
	}
	older := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(matches[0], older, older); err != nil {
		t.Fatal(err)
	}
	before := g.requests
	_, err = g.get(context.Background(), "cloud", "https://lichess.org/api/cloud-eval", query)
	if err != nil {
		t.Fatal(err)
	}
	if g.requests != before+1 {
		t.Fatalf("expired negative cache was not retried: %d -> %d", before, g.requests)
	}
}
