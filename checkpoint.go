package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// Checkpoints never contain a Lichess token. The checkpoint is keyed by the
// complete generation configuration, except for the resume toggle itself.
type generationCheckpoint struct {
	Report    *Report         `json:"report"`
	Queue     []int           `json:"queue"`
	Seen      map[string]int  `json:"seen"`
	Expanded  map[string]bool `json:"expanded"`
	Processed int             `json:"processed"`
	Stage     string          `json:"stage"`
	EvalIndex int             `json:"eval_index"`
}

func (g *generator) checkpointPath() string {
	cfg := g.c
	cfg.Resume = false
	raw, _ := json.Marshal(cfg)
	digest := sha256.Sum256(raw)
	return filepath.Join(g.cache, "checkpoints", hex.EncodeToString(digest[:])+".json")
}

func (g *generator) saveCheckpoint(c generationCheckpoint) error {
	return writeJSONAtomic(g.checkpointPath(), c)
}

func (g *generator) readCheckpoint() (generationCheckpoint, bool) {
	var cp generationCheckpoint
	raw, err := os.ReadFile(g.checkpointPath())
	if err != nil || json.Unmarshal(raw, &cp) != nil || cp.Report == nil || len(cp.Report.Nodes) == 0 || cp.Seen == nil || cp.Expanded == nil {
		return cp, false
	}
	if cp.Stage != "grow" && cp.Stage != "evaluate" {
		return cp, false
	}
	for _, id := range cp.Queue {
		if id < 0 || id >= len(cp.Report.Nodes) {
			return cp, false
		}
	}
	if cp.EvalIndex < 0 || cp.EvalIndex > len(cp.Report.Nodes) {
		return cp, false
	}
	return cp, true
}

func (g *generator) removeCheckpoint() { _ = os.Remove(g.checkpointPath()) }
