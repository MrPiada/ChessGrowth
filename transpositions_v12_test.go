package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestV12PositionKeyIgnoresClockButNotTurnAndCastling(t *testing.T) {
	a := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	b := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 7 9"
	if positionCacheKey(a) != positionCacheKey(b) {
		t.Fatal("move clocks are not part of position identity")
	}
	c := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 7 9"
	if positionCacheKey(a) == positionCacheKey(c) {
		t.Fatal("side to move matters")
	}
	d := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w Qkq - 7 9"
	if positionCacheKey(a) == positionCacheKey(d) {
		t.Fatal("castling rights matter")
	}
}

// Two move orders (1.Nf3 d5 2.g3 Nf6 / 1.Nf3 Nf6 2.g3 d5)
// lead to one position. Only the canonical occurrence may be expanded.
func TestV12DeduplicatesTranspositionAndEngineCalls(t *testing.T) {
	c := defaults()
	c.Color = "white"
	c.EngineMode = "local"
	c.MaxDepth = 4
	c.MaxNodes = 25
	c.MinGames = 1
	c.Coverage = 95
	c.DelayMS = 300
	g := newGenerator(c, nil)
	g.cache = t.TempDir()
	g.token = ""
	enginePositions := map[string]int{}
	g.stockfishFn = func(_ context.Context, _ string, fen string, _ int) (int, error) {
		enginePositions[positionCacheKey(fen)]++
		return 27, nil
	}
	explorerPositions := map[string]int{}
	g.client = &http.Client{Transport: mockTransportV1(func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() != "explorer.lichess.org" {
			return fakeResponse(500, "unexpected host"), nil
		}
		fen := req.URL.Query().Get("fen")
		board, err := ParseFEN(fen)
		if err != nil {
			return fakeResponse(400, err.Error()), nil
		}
		explorerPositions[board.Key()]++
		legal := map[string]bool{}
		for _, m := range board.moves() {
			legal[moveUCI(m)] = true
		}
		moves := []map[string]any{}
		add := func(uci string, games int) {
			if !legal[uci] {
				t.Errorf("illegal fixture move %s in %s", uci, fen)
				return
			}
			moves = append(moves, map[string]any{"uci": uci, "white": games, "draws": 0, "black": 0})
		}
		switch board.Ply() {
		case 0:
			add("g1f3", 1000)
		case 1:
			add("d7d5", 600)
			add("g8f6", 400)
		case 2:
			add("g2g3", 1000)
		case 3:
			if strings.Contains(strings.Fields(fen)[0], "3p4") {
				add("g8f6", 1000)
			} else {
				add("d7d5", 1000)
			}
		}
		out, _ := json.Marshal(map[string]any{"white": 1000, "draws": 0, "black": 0, "moves": moves})
		return fakeResponse(200, string(out)), nil
	})}
	report, err := g.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	duplicate := 0
	seen := map[string]bool{}
	for _, node := range report.Nodes {
		key := positionCacheKey(node.FEN)
		if node.Transposition {
			duplicate++
			if node.TranspositionTo == nil {
				t.Errorf("transposition node %d has no canonical link", node.ID)
			}
			if node.EvalCP == nil {
				t.Errorf("transposition node %d lost its evaluation", node.ID)
			}
			if len(node.Children) != 0 {
				t.Errorf("duplicate node %d was expanded", node.ID)
			}
		} else if seen[key] {
			t.Errorf("unique position expanded twice: node %d", node.ID)
		}
		seen[key] = true
	}
	if duplicate != 1 || g.transpositions != 1 {
		t.Fatalf("wanted exactly one transposition, got %d (%d hits); nodes=%d", duplicate, g.transpositions, len(report.Nodes))
	}
	for key, count := range enginePositions {
		if count != 1 {
			t.Errorf("engine called %d times for %s", count, key)
		}
	}
	for key, count := range explorerPositions {
		if count != 1 {
			t.Errorf("explorer called %d times for %s", count, key)
		}
	}
	if !strings.Contains(report.ExportPGN(), "Transposition: same position as node") {
		t.Fatal("PGN missing transposition link annotation")
	}
	t.Logf("%d move nodes, %d unique positions, %d saved expansion(s), %d HTTP calls", len(report.Nodes), len(enginePositions), duplicate, g.requests)
}

func TestV12LivePreviewKeepsCurrentWhenLarge(t *testing.T) {
	c := defaults()
	g := newGenerator(c, nil)
	r := newReport(c)
	for i := 0; i < 400; i++ {
		parent := -1
		if i > 0 {
			parent = (i - 1) / 2
		}
		r.Nodes = append(r.Nodes, &Node{ID: i, Parent: parent, Depth: i / 2, Ply: i + 1, SAN: "Nf3", FEN: initialFEN, Children: []int{}})
	}
	called := false
	g.onLive = func(ns []LiveNode, processed, pending, current int) {
		called = true
		got := map[int]bool{}
		for _, n := range ns {
			got[n.ID] = true
		}
		if !got[0] || !got[399] || !got[199] {
			t.Fatalf("live preview lost root/current/ancestor: %d nodes", len(ns))
		}
		if len(ns) > 250 {
			t.Fatalf("live sampling unexpectedly large: %d", len(ns))
		}
		for _, n := range ns {
			if n.Parent >= 0 && !got[n.Parent] {
				t.Fatalf("orphan node %d", n.ID)
			}
		}
	}
	g.emitLive(r, 12, 4, 399, true)
	if !called {
		t.Fatal("no live update")
	}
}
