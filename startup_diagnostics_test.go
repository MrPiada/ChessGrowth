package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// A slow Explorer response must not leave the UI saying "0 API requests".
func TestStartupProgressIsVisibleWhileExplorerRequestIsPending(t *testing.T) {
	t.Setenv("LICHESS_TOKEN", "lip_local_test_token")
	cfg := defaults()
	cfg.EngineMode = "cloud"
	cfg.MaxDepth = 1
	cfg.DelayMS = 300
	type event struct {
		nodes, requests int
		msg             string
	}
	events := make(chan event, 20)
	gen := newGenerator(cfg, func(nodes, requests int, msg string) {
		events <- event{nodes, requests, msg}
	})
	gen.cache = t.TempDir()
	entered := make(chan struct{})
	release := make(chan struct{})
	gen.client = &http.Client{Transport: mockTransportV1(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/lichess") {
			close(entered)
			<-release
			return fakeResponse(200, `{"white":0,"draws":0,"black":0,"moves":[]}`), nil
		}
		return fakeResponse(404, ""), nil
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := gen.run(ctx); finished <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("Explorer request was never started")
	}
	observed := false
	for len(events) > 0 {
		e := <-events
		if e.nodes >= 1 && e.requests == 1 && strings.Contains(e.msg, "explorer") {
			observed = true
		}
	}
	close(release)
	if !observed {
		t.Error("No progress update while initial Explorer request was pending")
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Generator did not finish after Explorer returned")
	}
}
