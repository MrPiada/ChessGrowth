package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestLivePositionSnapshotCarriesOnlyCurrentFEN(t *testing.T) {
	g := newGenerator(defaults(), nil)
	report := newReport(defaults())
	board, err := ParseFEN(initialFEN)
	if err != nil {
		t.Fatal(err)
	}
	root := report.add(-1, board, Move{}, "Starting position")
	moves := board.moves()
	if len(moves) == 0 {
		t.Fatal("no opening moves")
	}
	child := report.add(root.ID, board.Apply(moves[0]), moves[0], "")
	g.onLive = func(nodes []LiveNode, _, _, current int) {
		if current != child.ID {
			t.Fatalf("wrong active node %d", current)
		}
		found := false
		for _, n := range nodes {
			if n.ID == current {
				found = true
				if n.FEN != child.FEN || n.SAN != child.SAN {
					t.Fatalf("wrong position: %#v", n)
				}
			} else if n.FEN != "" {
				t.Fatalf("non-current node leaked FEN: %#v", n)
			}
		}
		if !found {
			t.Fatal("current node missing")
		}
	}
	g.emitLive(report, 0, 1, child.ID, true)
}

func TestLivePositionProgressAPI(t *testing.T) {
	a := newApplication()
	a.phase = "running"
	a.current = 12
	cp := -56
	a.live = []LiveNode{
		{ID: 0, FEN: "", SAN: "Start"},
		{ID: 12, FEN: initialFEN, SAN: "Nf3", Ply: 3, EvalCP: &cp, EvalSource: "Stockfish (local)"},
	}
	srv := httptest.NewServer(a.handlers())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/progress")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var payload struct {
		CurrentFEN    string `json:"current_fen"`
		CurrentMove   string `json:"current_move"`
		CurrentPly    int    `json:"current_ply"`
		CurrentEvalCP *int   `json:"current_eval_cp"`
		CurrentSource string `json:"current_eval_source"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.CurrentFEN != initialFEN || payload.CurrentMove != "Nf3" || payload.CurrentPly != 3 || payload.CurrentEvalCP == nil || *payload.CurrentEvalCP != -56 || payload.CurrentSource != "Stockfish (local)" {
		t.Fatalf("unexpected live payload: %#v", payload)
	}
}

func TestLivePositionGUIEmbedded(t *testing.T) {
	html, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	js, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("web/app.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"live-preview-layout", "live-position-board", "live-position-turn", "live-position-eval"} {
		if !strings.Contains(string(html), target) {
			t.Errorf("missing live UI element %q", target)
		}
	}
	if !strings.Contains(string(js), "boardMarkup(fen,state.liveBoardFlipped,false)") {
		t.Fatal("position preview must reuse fixed SVG renderer")
	}
	if !strings.Contains(string(css), ".live-preview-layout") {
		t.Fatal("missing responsive layout styles")
	}
}
