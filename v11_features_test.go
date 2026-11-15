package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestV11EveryNodeEvaluatedAndPGNAnnotated(t *testing.T) {
	c := defaults()
	c.EngineMode = "hybrid"
	c.Color = "white"
	c.MaxDepth = 2
	c.MaxNodes = 8
	c.Coverage = 80
	c.MinGames = 1
	c.DelayMS = 300
	g := newGenerator(c, nil)
	g.cache = t.TempDir()
	g.token = ""
	localCalls := 0
	g.stockfishFn = func(_ context.Context, _ string, fen string, _ int) (int, error) {
		localCalls++
		b, e := ParseFEN(fen)
		if e != nil {
			return 0, e
		}
		return b.Ply() * 200, nil
	}
	g.client = &http.Client{Transport: mockTransportV1(func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() == "lichess.org" {
			return fakeResponse(404, ""), nil
		}
		b, e := ParseFEN(req.URL.Query().Get("fen"))
		if e != nil {
			t.Error(e)
			return fakeResponse(500, ""), nil
		}
		if b.Ply() == 0 {
			return fakeResponse(200, `{"white":900,"draws":50,"black":50,"opening":{"eco":"A00","name":"King's Pawn Game"},"moves":[{"uci":"e2e4","white":800,"draws":40,"black":40,"opening":{"eco":"B00","name":"King's Pawn Opening"}}]}`), nil
		}
		if b.Ply() == 1 {
			return fakeResponse(200, `{"white":800,"draws":40,"black":40,"moves":[{"uci":"e7e5","white":650,"draws":30,"black":30,"opening":{"eco":"C20","name":"King's Pawn Game"}}]}`), nil
		}
		return fakeResponse(200, `{"white":650,"draws":30,"black":30,"moves":[]}`), nil
	})}
	snapshots := 0
	g.onLive = func(_ []LiveNode, _, _, _ int) { snapshots++ }
	r, e := g.run(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Nodes) != 3 {
		t.Fatalf("unexpected nodes %d", len(r.Nodes))
	}
	for _, n := range r.Nodes {
		if n.EvalCP == nil || n.EvalSource != "Stockfish (local)" {
			t.Errorf("node %d unscored: %#v", n.ID, n)
		}
	}
	if localCalls != 3 {
		t.Fatalf("expected root and both descendants evaluated, got %d", localCalls)
	}
	if r.Nodes[1].OpeningName != "King's Pawn Opening" || r.Nodes[2].OpeningECO != "C20" {
		t.Fatalf("opening names missing: %#v", r.Nodes[2])
	}
	if r.Nodes[1].TrapStatus != "yes" || r.Nodes[2].TrapStatus != "yes" {
		t.Errorf("expected heuristic trap %s / %s", r.Nodes[1].TrapStatus, r.Nodes[2].TrapStatus)
	}
	if snapshots < 2 {
		t.Error("no live snapshots generated")
	}
	pgn := r.ExportPGN()
	for _, frag := range []string{"[%eval 2.00]", "[%eval 4.00]", "Lichess W/D/B:", "Trap: yes", "Opening: C20 King's Pawn Game"} {
		if !strings.Contains(pgn, frag) {
			t.Errorf("missing %q in PGN:\n%s", frag, pgn)
		}
	}
}
func TestV11VectorPiecesAndProgressAPI(t *testing.T) {
	app := newApplication()
	srv := httptest.NewServer(app.handlers())
	defer srv.Close()
	res, e := http.Get(srv.URL + "/assets/pieces/wK.svg")
	if e != nil {
		t.Fatal(e)
	}
	data, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(data), "<svg") {
		t.Fatalf("vector piece not embedded: %d", res.StatusCode)
	}
	res, e = http.Get(srv.URL + "/api/progress")
	if e != nil {
		t.Fatal(e)
	}
	data, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(data), `"pending":0`) {
		t.Fatalf("progress response invalid: %s", data)
	}
	if _, e = os.Stat("assets/pieces/bN.svg"); e != nil {
		t.Fatal(e)
	}
}
