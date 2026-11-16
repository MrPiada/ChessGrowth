package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCloud429FallsBackImmediatelyAndPersistsCooldown(t *testing.T) {
	t.Setenv("LICHESS_TOKEN", "lip_unit_test_token")
	c := defaults()
	c.EngineMode = "hybrid"
	dir := t.TempDir()
	remoteCalls, localCalls := 0, 0
	makeGen := func() *generator {
		g := newGenerator(c, nil)
		g.cache = dir
		g.stockfishFn = func(ctx context.Context, path, fen string, depth int) (int, error) {
			localCalls++
			return 45, nil
		}
		g.client = &http.Client{Transport: mockTransportV1(func(r *http.Request) (*http.Response, error) {
			remoteCalls++
			resp := fakeResponse(429, `rate limited`)
			resp.Header.Set("Retry-After", "120")
			return resp, nil
		})}
		return g
	}
	g := makeGen()
	started := time.Now()
	first, err := g.eval(context.Background(), initialFEN)
	if err != nil || first.cp == nil || *first.cp != 45 {
		t.Fatalf("fallback: %+v %v", first, err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("Cloud retry blocked the local fallback")
	}
	if remoteCalls != 1 || localCalls != 1 {
		t.Fatalf("requests=%d local=%d", remoteCalls, localCalls)
	}
	if !g.cloudDisabled || time.Until(g.cooldown.Until) < 100*time.Second {
		t.Fatal("429 did not open persistent circuit")
	}
	// A different position is evaluated locally without re-querying Cloud.
	secondFen := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"
	_, err = g.eval(context.Background(), secondFen)
	if err != nil || remoteCalls != 1 || localCalls != 2 {
		t.Fatalf("breaker ignored: %v / %d / %d", err, remoteCalls, localCalls)
	}
	// New process: cooldown survives restart; locally cached Stockfish score is reused.
	g2 := makeGen()
	_, err = g2.eval(context.Background(), secondFen)
	if err != nil || remoteCalls != 1 || localCalls != 2 {
		t.Fatalf("disk cache missing: %v / %d / %d", err, remoteCalls, localCalls)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "rate-limit.json"))
	if err != nil || !json.Valid(raw) || strings.Contains(string(raw), "lip_") {
		t.Fatal("cooldown file missing, malformed or contains credentials")
	}
}

func TestLocalEvaluationDiskCacheInvalidatedByDepth(t *testing.T) {
	c := defaults()
	c.EngineMode = "local"
	dir := t.TempDir()
	calls := 0
	makeGen := func(depth int) *generator {
		c.StockfishDepth = depth
		g := newGenerator(c, nil)
		g.cache = dir
		g.stockfishFn = func(_ context.Context, _, _ string, _ int) (int, error) { calls++; return 33, nil }
		return g
	}
	if _, err := makeGen(12).eval(context.Background(), initialFEN); err != nil {
		t.Fatal(err)
	}
	if _, err := makeGen(12).eval(context.Background(), initialFEN); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("cache miss: %d", calls)
	}
	if _, err := makeGen(14).eval(context.Background(), initialFEN); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("depth should invalidate score: %d", calls)
	}
}

func TestCheckpointsCanBeResumedAndHaveNoCredentials(t *testing.T) {
	c := defaults()
	c.Resume = true
	g := newGenerator(c, nil)
	g.cache = t.TempDir()
	board, err := ParseFEN(initialFEN)
	if err != nil {
		t.Fatal(err)
	}
	r := newReport(c)
	r.add(-1, board, Move{}, "Starting position")
	cp := generationCheckpoint{Report: r, Queue: []int{0}, Seen: map[string]int{board.Key(): 0}, Expanded: map[string]bool{}, Processed: 0, Stage: "grow"}
	if err := g.saveCheckpoint(cp); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(g.checkpointPath())
	if err != nil || !json.Valid(data) || strings.Contains(string(data), "lip_") {
		t.Fatal("invalid checkpoint")
	}
	again := newGenerator(c, nil)
	again.cache = g.cache
	restored, ok := again.readCheckpoint()
	if !ok || len(restored.Queue) != 1 || len(restored.Report.Nodes) != 1 {
		t.Fatal("checkpoint not restored")
	}
	again.removeCheckpoint()
	if _, ok = again.readCheckpoint(); ok {
		t.Fatal("checkpoint not deleted")
	}
}

func TestRetryAfterRespectsMinimumAndDate(t *testing.T) {
	now := time.Now().UTC()
	hdr := make(http.Header)
	hdr.Set("Retry-After", "20")
	if retryAfter(hdr, now) != time.Minute {
		t.Fatal("must wait >= 1 minute")
	}
	hdr.Set("Retry-After", "180")
	if retryAfter(hdr, now) != 3*time.Minute {
		t.Fatal("must respect header")
	}
	hdr.Set("Retry-After", now.Add(10*time.Minute).Format(http.TimeFormat))
	if retryAfter(hdr, now) < 9*time.Minute {
		t.Fatal("must parse HTTP date")
	}
}

func TestInterruptedGenerationResumesWithoutRepeatingCompletedNodes(t *testing.T) {
	t.Setenv("LICHESS_TOKEN", "lip_test_resume_token")
	c := defaults()
	c.EngineMode = "practical"
	c.MaxDepth = 3
	c.MaxNodes = 20
	c.Resume = true
	dir := t.TempDir()
	g := newGenerator(c, nil)
	g.cache = dir
	calls := 0
	g.client = &http.Client{Transport: mockTransportV1(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return fakeResponse(200, `{"white":20,"draws":3,"black":10,"moves":[{"uci":"e2e4","san":"e4","white":12,"draws":2,"black":6}]}`), nil
		}
		return fakeResponse(403, `temporarily unavailable`), nil
	})}
	first, err := g.run(context.Background())
	if err == nil || first == nil || len(first.Nodes) != 2 {
		t.Fatalf("expected one completed expansion and an error: %v nodes %d", err, len(first.Nodes))
	}
	cp, ok := g.readCheckpoint()
	if !ok || cp.Processed != 1 || len(cp.Queue) != 1 {
		t.Fatalf("checkpoint not saved at completed task boundary: %+v %v", cp, ok)
	}
	next := newGenerator(c, nil)
	next.cache = dir
	newCalls := 0
	next.client = &http.Client{Transport: mockTransportV1(func(req *http.Request) (*http.Response, error) {
		newCalls++
		return fakeResponse(200, `{"white":0,"draws":0,"black":0,"moves":[]}`), nil
	})}
	completed, err := next.run(context.Background())
	if err != nil || len(completed.Nodes) != 2 || newCalls != 1 {
		t.Fatalf("resume failed: error=%v nodes=%d requests=%d", err, len(completed.Nodes), newCalls)
	}
	if _, ok = next.readCheckpoint(); ok {
		t.Fatal("finished run should remove checkpoint")
	}
}
