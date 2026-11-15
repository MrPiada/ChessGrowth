package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBoardAndOpeningSequence(t *testing.T) {
	b, e := ParseFEN(initialFEN)
	if e != nil {
		t.Fatal(e)
	}
	for _, san := range []string{"e4", "c5", "Nf3", "d6", "d4", "cxd4", "Nxd4"} {
		m, e := b.ParseSAN(san)
		if e != nil {
			t.Fatalf("%s: %v", san, e)
		}
		b = b.Apply(m)
	}
	if b.White {
		t.Fatal("expected black to move")
	}
	if _, e := b.ParseSAN("e4"); e == nil {
		t.Fatal("illegal SAN accepted")
	}
	if len(b.moves()) == 0 {
		t.Fatal("legal replies missing")
	}
}
func TestPruneUndoAndPGN(t *testing.T) {
	r := sampleReportForTest()
	before := r.activeCount()
	if before < 30 {
		t.Fatalf("demo too small: %d", before)
	}
	pgn := r.ExportPGN()
	if !strings.Contains(pgn, "1. e4") || !strings.Contains(pgn, "1. d4") || !strings.Contains(pgn, "( 1. d4") {
		t.Fatalf("missing mainline and root alternatives in PGN:\n%s", pgn[:min(len(pgn), 600)])
	}
	e4 := -1
	for _, id := range r.Nodes[0].Children {
		if r.Nodes[id].SAN == "e4" {
			e4 = id
		}
	}
	if e4 < 0 {
		t.Fatal("e4 branch not found")
	}
	removed, e := r.prune(e4)
	if e != nil {
		t.Fatal(e)
	}
	if len(removed) < 5 {
		t.Fatalf("expected subtree, removed %d", len(removed))
	}
	if r.activeCount() != before-len(removed) {
		t.Fatal("active node count incorrect")
	}
	after := r.ExportPGN()
	if strings.Contains(after, "1. e4") {
		t.Fatal("pruned e4 branch still present in PGN")
	}
	if !strings.Contains(after, "1. d4") {
		t.Fatal("other root branches removed accidentally")
	}
	r.restore(removed)
	if r.activeCount() != before {
		t.Fatal("undo did not restore all nodes")
	}
	if r.ExportPGN() != pgn {
		t.Fatal("PGN changed after undo")
	}
	if _, e = r.prune(0); e == nil {
		t.Fatal("root pruning should fail")
	}
}
func TestAPIPositionAndPrune(t *testing.T) {
	app := newApplication()
	srv := httptest.NewServer(app.handlers())
	defer srv.Close()
	res, e := http.Get(srv.URL + "/api/board")
	if e != nil {
		t.Fatal(e)
	}
	var board struct{ Legal []struct{ UCI string } }
	if e = json.NewDecoder(res.Body).Decode(&board); e != nil {
		t.Fatal(e)
	}
	_ = res.Body.Close()
	if len(board.Legal) != 20 {
		t.Fatalf("starting position has %d moves, want 20", len(board.Legal))
	}
	req, e := http.NewRequest("POST", srv.URL+"/api/sequence", strings.NewReader(`{"fen":"`+initialFEN+`","moves":"1. e4 e5 2. Nf3"}`))
	if e != nil {
		t.Fatal(e)
	}
	res, e = http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("sequence HTTP %d: %s", res.StatusCode, body)
	}
	if !strings.Contains(string(body), "5N2") {
		t.Fatalf("unexpected sequence response: %s", body)
	}
	app.mu.Lock()
	app.result = sampleReportForTest()
	app.phase = "done"
	app.mu.Unlock()
	res, e = http.Post(srv.URL+"/api/prune", "application/json", strings.NewReader(`{"id":1}`))
	if e != nil {
		t.Fatal(e)
	}
	if res.StatusCode != 200 {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("prune failed: %s", raw)
	}
	_ = res.Body.Close()
	res, e = http.Post(srv.URL+"/api/undo", "application/json", strings.NewReader(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	_ = res.Body.Close()
}

type fakeTransport func(*http.Request) (*http.Response, error)

func (f fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
func TestGeneratorWithMockedLichess(t *testing.T) {
	c := defaults()
	c.EngineMode = "practical"
	c.MaxDepth = 2
	c.MaxNodes = 10
	c.MinGames = 1
	c.Coverage = 80
	c.DelayMS = 300
	c.Color = "white"
	g := newGenerator(c, func(int, int, string) {})
	g.cache = t.TempDir()
	g.client = &http.Client{Transport: fakeTransport(func(req *http.Request) (*http.Response, error) {
		b, e := ParseFEN(req.URL.Query().Get("fen"))
		if e != nil {
			t.Fatal(e)
		}
		total := map[string]any{}
		if b.Ply() == 0 {
			total = map[string]any{"white": 700, "draws": 100, "black": 200, "moves": []any{map[string]any{"uci": "e2e4", "white": 530, "draws": 100, "black": 70}, map[string]any{"uci": "d2d4", "white": 90, "draws": 0, "black": 210}}}
		}
		if b.Ply() == 1 {
			total = map[string]any{"white": 500, "draws": 200, "black": 300, "moves": []any{map[string]any{"uci": "e7e5", "white": 350, "draws": 120, "black": 230}, map[string]any{"uci": "c7c5", "white": 150, "draws": 80, "black": 70}}}
		}
		raw, _ := json.Marshal(total)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(raw))), Request: req}, nil
	})}
	r, e := g.run(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Nodes) != 4 {
		t.Fatalf("got %d nodes, expected root + e4 + 2 black replies", len(r.Nodes))
	}
	if r.Nodes[1].SAN != "e4" {
		t.Fatalf("player chose %s, expected e4", r.Nodes[1].SAN)
	}
	if len(r.Nodes[1].Children) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(r.Nodes[1].Children))
	}
}
