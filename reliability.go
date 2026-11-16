package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The Cloud API is best-effort: a 429 disables Cloud for the *current run*.
// All Lichess services share a persisted cooldown to respect Lichess's request policy.
var errCloudCoolingDown = errors.New("Lichess Cloud temporarily unavailable (rate limited); local engine fallback")

type apiCooldown struct {
	Until      time.Time `json:"until"`
	CloudUntil time.Time `json:"cloud_until"`
}

func (g *generator) loadCooldown() {
	if g.cooldownLoaded {
		return
	}
	g.cooldownLoaded = true
	raw, err := os.ReadFile(filepath.Join(g.cache, "rate-limit.json"))
	if err == nil {
		_ = json.Unmarshal(raw, &g.cooldown)
	}
	if time.Now().Before(g.cooldown.CloudUntil) {
		g.cloudDisabled = true
	}
}

// Retry-After accepts seconds and HTTP dates. Never resume Lichess requests
// sooner than the officially recommended one minute after a 429.
func retryAfter(h http.Header, now time.Time) time.Duration {
	d := time.Minute
	if seconds, err := strconv.Atoi(strings.TrimSpace(h.Get("Retry-After"))); err == nil && seconds > 0 {
		if time.Duration(seconds)*time.Second > d {
			d = time.Duration(seconds) * time.Second
		}
	} else if target, err := http.ParseTime(h.Get("Retry-After")); err == nil {
		if target.Sub(now) > d {
			d = target.Sub(now)
		}
	}
	if d > 24*time.Hour {
		d = 24 * time.Hour
	}
	return d
}

func (g *generator) recordRateLimit(service string, headers http.Header) time.Duration {
	g.loadCooldown()
	now := time.Now()
	d := retryAfter(headers, now)
	until := now.Add(d)
	if until.After(g.cooldown.Until) {
		g.cooldown.Until = until
	}
	if service == "cloud" {
		g.cloudDisabled = true
		if until.After(g.cooldown.CloudUntil) {
			g.cooldown.CloudUntil = until
		}
	}
	// The cooldown file never contains a token or raw URL.
	_ = writeJSONAtomic(filepath.Join(g.cache, "rate-limit.json"), g.cooldown)
	return d
}

func (g *generator) waitForLichess(ctx context.Context, service string) error {
	g.loadCooldown()
	if service == "cloud" && g.cloudDisabled {
		return errCloudCoolingDown
	}
	delta := time.Until(g.cooldown.Until)
	if delta <= 0 {
		return nil
	}
	if service == "cloud" {
		return errCloudCoolingDown
	}
	g.progress(g.progressNodes, fmt.Sprintf("Lichess rate limit · %s requests paused for %s", service, delta.Round(time.Second)))
	timer := time.NewTimer(delta)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		case <-ticker.C:
			remaining := time.Until(g.cooldown.Until)
			if remaining > 0 {
				g.progress(g.progressNodes, fmt.Sprintf("Lichess API cooldown · %s paused for %s", service, remaining.Round(time.Second)))
			}
		}
	}
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".chessgrowth-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := json.NewEncoder(tmp).Encode(value); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

type evalDiskRecord struct {
	CP              *int   `json:"cp"`
	Mate            *int   `json:"mate,omitempty"`
	Source          string `json:"source"`
	Depth           int    `json:"depth"`
	EngineSignature string `json:"engine_signature"`
}

func (g *generator) engineIdentity() (string, error) {
	if g.stockfishFn != nil {
		return "injected-test-engine", nil
	}
	exe, err := findStockfish(g.c.StockfishPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(exe)
	if err != nil {
		return "", err
	}
	resolved, _ := filepath.Abs(exe)
	// Size + mtime invalidates cached scores after a binary update without
	// expensive rehashing of a potentially large Stockfish executable.
	return fmt.Sprintf("%s:%d:%d", resolved, info.Size(), info.ModTime().UnixNano()), nil
}

func (g *generator) evalDiskPath(fen string) (string, string, error) {
	engine, err := g.engineIdentity()
	if err != nil {
		return "", "", err
	}
	hash := sha256.Sum256([]byte(positionCacheKey(fen) + "\n" + engine + "\n" + strconv.Itoa(g.c.StockfishDepth)))
	return filepath.Join(g.cache, "evaluations", hex.EncodeToString(hash[:])+".json"), engine, nil
}

func (g *generator) readLocalEvaluation(fen string) (evalResult, bool) {
	path, engine, err := g.evalDiskPath(fen)
	if err != nil {
		return evalResult{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return evalResult{}, false
	}
	var record evalDiskRecord
	if json.Unmarshal(raw, &record) != nil || record.CP == nil || record.EngineSignature != engine || record.Depth != g.c.StockfishDepth {
		return evalResult{}, false
	}
	return evalResult{cp: record.CP, mate: record.Mate, src: record.Source}, true
}

func (g *generator) writeLocalEvaluation(fen string, x evalResult) {
	if x.cp == nil {
		return
	}
	path, engine, err := g.evalDiskPath(fen)
	if err != nil {
		return
	}
	_ = writeJSONAtomic(path, evalDiskRecord{CP: x.cp, Mate: x.mate, Source: x.src, Depth: g.c.StockfishDepth, EngineSignature: engine})
}
