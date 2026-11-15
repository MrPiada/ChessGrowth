package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	StartFEN        string   `json:"start_fen"`
	Color           string   `json:"color"`
	Speeds          []string `json:"speeds"`
	Ratings         []int    `json:"ratings"`
	MaxDepth        int      `json:"max_depth"`
	MaxNodes        int      `json:"max_nodes"`
	Coverage        float64  `json:"coverage"`
	MinGames        int64    `json:"min_games"`
	EngineLines     int      `json:"engine_lines"`
	EngineThreshold float64  `json:"engine_threshold"`
	EngineMode      string   `json:"engine_mode"`
	StockfishPath   string   `json:"stockfish_path"`
	StockfishDepth  int      `json:"stockfish_depth"`
	Candidates      int      `json:"candidates"`
	DelayMS         int      `json:"delay_ms"`
	Event           string   `json:"event"`
}

func defaults() Config {
	return Config{StartFEN: initialFEN, Color: "white", Speeds: []string{"blitz", "rapid"}, Ratings: []int{1800, 2000, 2200}, MaxDepth: 12, MaxNodes: 250, Coverage: 85, MinGames: 10, EngineLines: 3, EngineThreshold: 0.8, EngineMode: "cloud", StockfishDepth: 16, Candidates: 10, DelayMS: 600, Event: "ChessGrowth"}
}
func (c *Config) validate() error {
	if c.StartFEN == "" {
		c.StartFEN = initialFEN
	}
	b, e := ParseFEN(c.StartFEN)
	if e != nil {
		return e
	}
	if e = validBoard(b); e != nil {
		return e
	}
	c.StartFEN = b.FEN()
	if c.Color != "white" && c.Color != "black" {
		return errors.New("color must be white or black")
	}
	if c.MaxDepth < 1 || c.MaxDepth > 50 {
		return errors.New("max depth must be 1–50 half-moves")
	}
	if c.MaxNodes < 2 || c.MaxNodes > 5000 {
		return errors.New("max nodes must be 2–5000")
	}
	if c.Coverage <= 0 || c.Coverage > 100 {
		return errors.New("coverage must be >0 and <=100")
	}
	if c.MinGames < 0 {
		return errors.New("min games must be nonnegative")
	}
	if c.EngineLines < 1 || c.EngineLines > 20 {
		return errors.New("engine lines must be 1–20")
	}
	if c.EngineThreshold < 0 || c.EngineThreshold > 20 {
		return errors.New("engine threshold must be 0–20 pawns")
	}
	if c.Candidates < 1 || c.Candidates > 40 {
		return errors.New("candidate limit must be 1–40")
	}
	if c.DelayMS < 300 || c.DelayMS > 20000 {
		return errors.New("request delay must be 300–20000 ms")
	}
	if c.StockfishDepth < 5 || c.StockfishDepth > 35 {
		return errors.New("Stockfish depth must be 5–35")
	}
	if !map[string]bool{"cloud": true, "hybrid": true, "local": true, "practical": true}[c.EngineMode] {
		return errors.New("invalid evaluation mode")
	}
	if c.EngineMode == "local" || c.EngineMode == "hybrid" {
		if _, e := findStockfish(c.StockfishPath); e != nil {
			return e
		}
	}
	allowedSpeed := map[string]bool{"ultraBullet": true, "bullet": true, "blitz": true, "rapid": true, "classical": true, "correspondence": true}
	if len(c.Speeds) == 0 || len(c.Ratings) == 0 {
		return errors.New("choose at least one speed and one rating")
	}
	for _, s := range c.Speeds {
		if !allowedSpeed[s] {
			return fmt.Errorf("unsupported speed %s", s)
		}
	}
	allowedRating := map[int]bool{1600: true, 1800: true, 2000: true, 2200: true, 2500: true}
	for _, r := range c.Ratings {
		if !allowedRating[r] {
			return fmt.Errorf("unsupported rating %d", r)
		}
	}
	if len(c.Event) > 80 {
		return errors.New("event name too long")
	}
	if c.Event == "" {
		c.Event = "ChessGrowth"
	}
	return nil
}

type Node struct {
	ID               int     `json:"id"`
	Parent           int     `json:"parent"`
	Children         []int   `json:"children"`
	SAN              string  `json:"san"`
	UCI              string  `json:"uci"`
	FEN              string  `json:"fen"`
	Ply              int     `json:"ply"`
	Depth            int     `json:"depth"`
	Games            int64   `json:"games"`
	Frequency        float64 `json:"frequency"`
	White            int64   `json:"white"`
	Draws            int64   `json:"draws"`
	Black            int64   `json:"black"`
	EvalCP           *int    `json:"eval_cp"`
	EvalMate         *int    `json:"eval_mate,omitempty"`
	EvalSource       string  `json:"eval_source,omitempty"`
	EvalError        string  `json:"eval_error,omitempty"`
	OpeningName      string  `json:"opening_name,omitempty"`
	OpeningECO       string  `json:"opening_eco,omitempty"`
	OpeningInherited bool    `json:"opening_inherited,omitempty"`
	TrapStatus       string  `json:"trap_status"`
	TrapReason       string  `json:"trap_reason,omitempty"`
	Reason           string  `json:"reason,omitempty"`
	Transposition    bool    `json:"transposition,omitempty"`
	TranspositionTo  *int    `json:"transposition_to,omitempty"`
	Deleted          bool    `json:"deleted"`
}
type Report struct {
	Config    Config   `json:"config"`
	Nodes     []*Node  `json:"nodes"`
	Started   string   `json:"started"`
	Finished  string   `json:"finished"`
	Warnings  []string `json:"warnings"`
	Truncated bool     `json:"truncated"`
}

func newReport(c Config) *Report {
	return &Report{Config: c, Nodes: []*Node{}, Warnings: []string{}, Started: time.Now().UTC().Format(time.RFC3339)}
}
func (r *Report) add(parent int, b Board, m Move, reason string) *Node {
	n := &Node{ID: len(r.Nodes), Parent: parent, Children: []int{}, FEN: b.FEN(), Ply: b.Ply(), SAN: "Start", Depth: 0, Reason: reason}
	if parent >= 0 {
		p := r.Nodes[parent]
		prev, e := ParseFEN(p.FEN)
		if e == nil {
			n.SAN = prev.SAN(m)
		}
		n.UCI = moveUCI(m)
		n.Depth = p.Depth + 1
		p.Children = append(p.Children, n.ID)
	}
	r.Nodes = append(r.Nodes, n)
	return n
}
func (r *Report) activeCount() int {
	n := 0
	for _, p := range r.Nodes {
		if !p.Deleted {
			n++
		}
	}
	return n
}
func (r *Report) activeLeaves() int {
	n := 0
	for _, p := range r.Nodes {
		if p.Deleted {
			continue
		}
		c := 0
		for _, id := range p.Children {
			if !r.Nodes[id].Deleted {
				c++
			}
		}
		if c == 0 && p.ID != 0 {
			n++
		}
	}
	return n
}
func (r *Report) prune(id int) ([]int, error) {
	if id <= 0 || id >= len(r.Nodes) {
		return nil, errors.New("select a non-root branch")
	}
	if r.Nodes[id].Deleted {
		return nil, errors.New("this branch is already removed")
	}
	ids := []int{}
	var visit func(int)
	visit = func(k int) {
		if r.Nodes[k].Deleted {
			return
		}
		r.Nodes[k].Deleted = true
		ids = append(ids, k)
		for _, t := range r.Nodes[k].Children {
			visit(t)
		}
	}
	visit(id)
	return ids, nil
}
func (r *Report) restore(ids []int) {
	for _, id := range ids {
		if id > 0 && id < len(r.Nodes) {
			r.Nodes[id].Deleted = false
		}
	}
}

type openingInfo struct {
	ECO  string `json:"eco"`
	Name string `json:"name"`
}
type explorerMove struct {
	UCI, SAN            string
	White, Draws, Black int64
	Opening             *openingInfo `json:"opening"`
}
type explorerData struct {
	White, Draws, Black int64
	Opening             *openingInfo `json:"opening"`
	Moves               []explorerMove
}
type cloudData struct {
	PVs []struct {
		CP   *int `json:"cp"`
		Mate *int `json:"mate"`
	} `json:"pvs"`
}
type evalResult struct {
	cp   *int
	mate *int
	src  string
}
type generator struct {
	c                      Config
	client                 *http.Client
	cache                  string
	last                   time.Time
	requests               int
	warnings               map[string]bool
	onProgress             func(int, int, string)
	onLive                 func([]LiveNode, int, int, int)
	evalMemo               map[string]evalResult
	explorerMemo           map[string]explorerData
	transpositions         int
	localCount, cloudCount int
	lastLive               time.Time
	token                  string
	stockfishFn            func(context.Context, string, string, int) (int, error)
}

func newGenerator(c Config, cb func(int, int, string)) *generator {
	dir, _ := os.UserCacheDir()
	if dir == "" {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "ChessGrowth", "api")
	_ = os.MkdirAll(dir, 0700)
	token, _, e := activeToken(newTokenStore())
	if e != nil {
		token = ""
	} // A corrupt/missing token must not block public APIs.
	return &generator{c: c, client: &http.Client{Timeout: 35 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && req.URL.Hostname() != via[0].URL.Hostname() {
			return errors.New("cross-host redirect blocked")
		}
		return nil
	}}, cache: dir, onProgress: cb, warnings: map[string]bool{}, token: token, evalMemo: map[string]evalResult{}, explorerMemo: map[string]explorerData{}}
}
func (g *generator) warn(s string) { g.warnings[s] = true }
func (g *generator) get(ctx context.Context, service, endpoint string, params url.Values) ([]byte, error) {
	cacheParams := url.Values{}
	for k, vals := range params {
		cacheParams[k] = append([]string(nil), vals...)
	}
	if fen := cacheParams.Get("fen"); fen != "" {
		cacheParams.Set("fen", positionCacheKey(fen))
	}
	cacheKey := sha256.Sum256([]byte(service + "\n" + cacheParams.Encode()))
	path := filepath.Join(g.cache, service+"_"+hex.EncodeToString(cacheKey[:])+".json")
	if data, e := os.ReadFile(path); e == nil {
		if string(data) == "null" {
			// Cloud evaluations may be added later: refresh negative cache after 24h.
			if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < 24*time.Hour {
				return nil, nil
			}
		} else {
			return data, nil
		}
		// Expired cloud miss: request the position again.
	}
	for attempt := 0; attempt < 4; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		delta := time.Duration(g.c.DelayMS)*time.Millisecond - time.Since(g.last)
		if !g.last.IsZero() && delta > 0 {
			timer := time.NewTimer(delta)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		g.last = time.Now()
		g.requests++
		req, e := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("User-Agent", "ChessGrowth/0.1 (+https://github.com/MrPiada/ChessGrowth)")
		// Never disclose the user's credential to explorer.lichess.org or any other host.
		if req.URL.Hostname() == "lichess.org" && g.token != "" {
			req.Header.Set("Authorization", "Bearer "+g.token)
		}
		resp, e := g.client.Do(req)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", service, e)
		}
		raw, e := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
		resp.Body.Close()
		if e != nil {
			return nil, e
		}
		switch resp.StatusCode {
		case 200:
			if !json.Valid(raw) {
				return nil, fmt.Errorf("invalid %s response", service)
			}
			_ = os.WriteFile(path, raw, 0600)
			return raw, nil
		case 404:
			if service == "cloud" {
				_ = os.WriteFile(path, []byte("null"), 0600)
				return nil, nil
			}
			return nil, fmt.Errorf("%s HTTP 404", service)
		case 429, 502, 503, 504:
			d := time.Duration(2<<attempt) * time.Second
			if resp.StatusCode == 429 {
				d = 60 * time.Second
			}
			if x, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && x > 0 && x < 120 {
				if resp.StatusCode == 429 {
					d = time.Duration(max(x, 60)) * time.Second
				} else {
					d = time.Duration(x) * time.Second
				}
			}
			g.warn(fmt.Sprintf("Lichess rate limit/service error (HTTP %d): requests retried.", resp.StatusCode))
			timer := time.NewTimer(d)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		default:
			return nil, fmt.Errorf("%s HTTP %d: %s", service, resp.StatusCode, string(raw[:min(len(raw), 170)]))
		}
	}
	return nil, fmt.Errorf("%s unavailable after retries", service)
}

// positionCacheKey includes only game-state fields that affect moves and position.
// Normalize en-passant through Board.FEN, which omits impossible EP captures.
func positionCacheKey(fen string) string {
	if b, err := ParseFEN(fen); err == nil {
		return b.Key()
	}
	return fen
}
func (g *generator) explorer(ctx context.Context, fen string) (explorerData, error) {
	key := positionCacheKey(fen)
	if g.explorerMemo == nil {
		g.explorerMemo = map[string]explorerData{}
	}
	if data, ok := g.explorerMemo[key]; ok {
		return data, nil
	}
	// The API receives the original FEN, preserving its move number.
	q := url.Values{"variant": {"standard"}, "fen": {fen}, "speeds": {strings.Join(g.c.Speeds, ",")}, "topGames": {"0"}, "recentGames": {"0"}}
	rs := []string{}
	for _, r := range g.c.Ratings {
		rs = append(rs, strconv.Itoa(r))
	}
	q.Set("ratings", strings.Join(rs, ","))
	var d explorerData
	raw, e := g.get(ctx, "explorer", "https://explorer.lichess.org/lichess", q)
	if e != nil {
		return d, e
	}
	e = json.Unmarshal(raw, &d)
	if e == nil {
		g.explorerMemo[key] = d
	}
	return d, e
}

// eval caches successful results per run, including Cloud misses.
func (g *generator) eval(ctx context.Context, fen string) (evalResult, error) {
	if g.evalMemo == nil {
		g.evalMemo = map[string]evalResult{}
	}
	key := positionCacheKey(fen)
	if x, ok := g.evalMemo[key]; ok {
		return x, nil
	}
	x, err := g.evalUncached(ctx, fen)
	if err == nil {
		g.evalMemo[key] = x
	}
	return x, err
}
func (g *generator) evalUncached(ctx context.Context, fen string) (evalResult, error) {
	if g.c.EngineMode == "practical" {
		return evalResult{}, nil
	}
	local := func() (evalResult, error) {
		g.localCount++
		if g.stockfishFn != nil { // Injectable evaluator for deterministic tests.
			cp, err := g.stockfishFn(ctx, g.c.StockfishPath, fen, g.c.StockfishDepth)
			if err != nil {
				return evalResult{}, err
			}
			return evalResult{cp: &cp, src: "Stockfish (local)"}, nil
		}
		cp, mate, err := stockfishEvalDetailed(ctx, g.c.StockfishPath, fen, g.c.StockfishDepth)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{cp: &cp, mate: mate, src: "Stockfish (local)"}, nil
	}
	if g.c.EngineMode == "local" {
		return local()
	}
	q := url.Values{"variant": {"standard"}, "fen": {fen}, "multiPv": {"1"}}
	g.cloudCount++
	raw, cloudErr := g.get(ctx, "cloud", "https://lichess.org/api/cloud-eval", q)
	if cloudErr == nil && len(raw) > 0 {
		var cloud cloudData
		if err := json.Unmarshal(raw, &cloud); err != nil {
			cloudErr = err
		} else if len(cloud.PVs) > 0 {
			pv := cloud.PVs[0]
			if pv.CP != nil {
				return evalResult{cp: pv.CP, src: "Lichess Cloud"}, nil
			}
			if pv.Mate != nil {
				v := 99999
				if *pv.Mate < 0 {
					v = -99999
				}
				return evalResult{cp: &v, mate: pv.Mate, src: "Lichess Cloud"}, nil
			}
		}
	}
	if ctx.Err() != nil {
		return evalResult{}, ctx.Err()
	}
	if g.c.EngineMode == "hybrid" {
		// Cloud has no cached score or is unavailable: ask local Stockfish.
		result, localErr := local()
		if localErr == nil {
			return result, nil
		}
		if cloudErr != nil {
			return evalResult{}, fmt.Errorf("cloud: %v; local: %w", cloudErr, localErr)
		}
		return evalResult{}, fmt.Errorf("cloud position not found; local: %w", localErr)
	}
	if cloudErr != nil {
		return evalResult{}, cloudErr
	}
	return evalResult{}, nil
}

func playerScore(c Config, move explorerMove) float64 {
	total := float64(move.White + move.Draws + move.Black)
	if total == 0 {
		return 0
	}
	win := move.White
	if c.Color == "black" {
		win = move.Black
	}
	return (float64(win) + float64(move.Draws)*0.5) / total
}
func chooseMoves(g *generator, ctx context.Context, board Board, data explorerData) ([]candidate, error) {
	total := data.White + data.Draws + data.Black
	if total == 0 {
		return nil, nil
	}
	valid := map[string]Move{}
	for _, m := range board.moves() {
		valid[moveUCI(m)] = m
	}
	candidates := []candidate{}
	for _, x := range data.Moves {
		m, ok := valid[x.UCI]
		if !ok {
			continue
		}
		n := x.White + x.Draws + x.Black
		if n < g.c.MinGames {
			continue
		}
		candidates = append(candidates, candidate{m: m, x: x, n: n, freq: float64(n) * 100 / float64(total), score: playerScore(g.c, x)})
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	playerTurn := (board.White && g.c.Color == "white") || (!board.White && g.c.Color == "black")
	sort.SliceStable(candidates, func(i, j int) bool {
		if playerTurn {
			if candidates[i].score != candidates[j].score {
				return candidates[i].score > candidates[j].score
			}
		}
		return candidates[i].n > candidates[j].n
	})
	if !playerTurn {
		selected := []candidate{}
		sum := 0.0
		for _, a := range candidates {
			selected = append(selected, a)
			sum += a.freq
			if sum >= g.c.Coverage {
				break
			}
		}
		return selected, nil
	}
	if len(candidates) > g.c.Candidates {
		candidates = candidates[:g.c.Candidates]
	}
	if g.c.EngineMode == "practical" {
		return candidates[:1], nil
	}
	evaluated := 0
	for i := range candidates {
		future := board.Apply(candidates[i].m)
		ev, e := g.eval(ctx, future.FEN())
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// Never disguise network/engine failure as an objective evaluation.
			g.warn("Some positions have no engine evaluation; practical-score fallback was used.")
			continue
		}
		candidates[i].cp = ev.cp
		candidates[i].mate = ev.mate
		candidates[i].src = ev.src
		if ev.cp != nil {
			evaluated++
		}
	}
	if evaluated == 0 {
		g.warn("No evaluations returned for some branches; move chosen by Lichess game statistics.")
		return candidates[:1], nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i].cp, candidates[j].cp
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		if g.c.Color == "white" {
			return *a > *b
		}
		return *a < *b
	})
	good := []candidate{}
	limit := min(g.c.EngineLines, len(candidates))
	for i := 0; i < limit; i++ {
		x := candidates[i]
		if x.cp == nil {
			continue
		}
		if g.c.Color == "white" && float64(*x.cp) < -100*g.c.EngineThreshold {
			continue
		}
		if g.c.Color == "black" && float64(*x.cp) > 100*g.c.EngineThreshold {
			continue
		}
		good = append(good, x)
	}
	if len(good) == 0 {
		g.warn("No move satisfied the configured engine threshold; the best available move was retained.")
		return candidates[:1], nil
	}
	// Among engine-acceptable moves, pick the strongest practical result as in the original project.
	sort.SliceStable(good, func(i, j int) bool {
		if good[i].score == good[j].score {
			return good[i].n > good[j].n
		}
		return good[i].score > good[j].score
	})
	return good[:1], nil
}

type candidate struct {
	m           Move
	x           explorerMove
	n           int64
	freq, score float64
	cp          *int
	src         string
	mate        *int
}

type LiveNode struct {
	ID              int    `json:"id"`
	Parent          int    `json:"parent"`
	Depth           int    `json:"depth"`
	Ply             int    `json:"ply"`
	SAN             string `json:"san"`
	Player          bool   `json:"player"`
	EvalCP          *int   `json:"eval_cp"`
	EvalMate        *int   `json:"eval_mate,omitempty"`
	EvalSource      string `json:"eval_source,omitempty"`
	TranspositionTo *int   `json:"transposition_to,omitempty"`
}

func isPlayerTurn(b Board, c Config) bool {
	return (b.White && c.Color == "white") || (!b.White && c.Color == "black")
}
func (g *generator) emitLive(r *Report, processed, pending, current int, force bool) {
	if g.onLive == nil {
		return
	}
	if !force && time.Since(g.lastLive) < 350*time.Millisecond {
		return
	}
	g.lastLive = time.Now()
	// A bounded, connected sample of the oldest and newest branches. Unlike a
	// fixed first-N preview, this keeps displaying progress after N nodes.
	selected := map[int]bool{0: true}
	addAncestors := func(id int) {
		for id >= 0 && id < len(r.Nodes) && !selected[id] {
			selected[id] = true
			id = r.Nodes[id].Parent
		}
	}
	for i := 1; i < len(r.Nodes) && i < 75; i++ {
		addAncestors(i)
	}
	for i, count := len(r.Nodes)-1, 0; i >= 1 && count < 90; i, count = i-1, count+1 {
		addAncestors(i)
	}
	if current >= 0 && current < len(r.Nodes) {
		addAncestors(current)
	}
	// The first positions retain historical context, while late branches remain visible.
	live := make([]LiveNode, 0, len(selected))
	for _, n := range r.Nodes {
		if !selected[n.ID] {
			continue
		}
		player := false
		if n.Parent >= 0 {
			if b, err := ParseFEN(r.Nodes[n.Parent].FEN); err == nil {
				player = isPlayerTurn(b, g.c)
			}
		}
		live = append(live, LiveNode{ID: n.ID, Parent: n.Parent, Depth: n.Depth, Ply: n.Ply, SAN: n.SAN, Player: player, EvalCP: n.EvalCP, EvalMate: n.EvalMate, EvalSource: n.EvalSource, TranspositionTo: n.TranspositionTo})
	}
	g.onLive(live, processed, pending, current)
}

func (g *generator) run(ctx context.Context) (*Report, error) {
	b, e := ParseFEN(g.c.StartFEN)
	if e != nil {
		return nil, e
	}
	r := newReport(g.c)
	r.add(-1, b, Move{}, "Starting position")
	// Record a score for the root, too: it is the baseline for detecting tactical swings.
	if g.c.EngineMode != "practical" {
		ev, e := g.eval(ctx, b.FEN())
		if e == nil {
			r.Nodes[0].EvalCP = ev.cp
			r.Nodes[0].EvalMate = ev.mate
			r.Nodes[0].EvalSource = ev.src
		}
		if e != nil {
			r.Nodes[0].EvalError = e.Error()
			g.warn("Some positions could not be evaluated; verify Stockfish and the Cloud fallback.")
		}
	}

	type task struct {
		id    int
		board Board
	}
	queue := []task{{0, b}}
	// Canonical positions form a position DAG. The report still stores move edges,
	// but reaching a known position adds only a transposition reference: never a
	// second expansion or another set of requests/evaluations.
	seen := map[string]int{b.Key(): 0}
	expanded := map[string]bool{}
	processed := 0
	g.emitLive(r, processed, len(queue), 0, true)
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		current := queue[0]
		queue = queue[1:]
		processed++
		n := r.Nodes[current.id]
		if g.onProgress != nil {
			g.onProgress(len(r.Nodes), g.requests, fmt.Sprintf("Exploring %s · depth %d · %d nodes", n.SAN, n.Depth, len(r.Nodes)))
		}
		g.emitLive(r, processed, len(queue), n.ID, false)
		if n.Depth >= g.c.MaxDepth {
			n.Reason = "Depth limit"
			continue
		}
		if len(current.board.moves()) == 0 {
			n.Reason = "Game over"
			continue
		}
		key := current.board.Key()
		if expanded[key] { // Safety guard for unusual restored/queued reports.
			if canonical, ok := seen[key]; ok && canonical != n.ID {
				n.Transposition = true
				n.TranspositionTo = &canonical
				n.Reason = fmt.Sprintf("Transposition to node #%d", canonical)
			}
			continue
		}
		expanded[key] = true
		data, err := g.explorer(ctx, current.board.FEN())
		if err != nil {
			return r, err
		}
		if data.Opening != nil && data.Opening.Name != "" {
			n.OpeningName = data.Opening.Name
			n.OpeningECO = data.Opening.ECO
			n.OpeningInherited = false
		}
		// The parent move's outcome sample is aligned with this queried position.
		n.White, n.Draws, n.Black = data.White, data.Draws, data.Black
		n.Games = data.White + data.Draws + data.Black
		if data.White+data.Black+data.Draws == 0 {
			n.Reason = "No Lichess games"
			continue
		}
		candidates, err := chooseMoves(g, ctx, current.board, data)
		if err != nil {
			return r, err
		}
		if len(candidates) == 0 {
			n.Reason = "No moves passing minimum sample"
			continue
		}
		for _, x := range candidates {
			if len(r.Nodes) >= g.c.MaxNodes {
				r.Truncated = true
				n.Reason = "Maximum node limit reached"
				break
			}
			after := current.board.Apply(x.m)
			child := r.add(n.ID, after, x.m, "")
			child.Games = x.n
			child.Frequency = math.Round(x.freq*10) / 10
			child.White = x.x.White
			child.Draws = x.x.Draws
			child.Black = x.x.Black
			child.EvalCP = x.cp
			child.EvalMate = x.mate
			child.EvalSource = x.src
			child.OpeningName = n.OpeningName
			child.OpeningECO = n.OpeningECO
			child.OpeningInherited = child.OpeningName != ""
			if x.x.Opening != nil && x.x.Opening.Name != "" {
				child.OpeningName = x.x.Opening.Name
				child.OpeningECO = x.x.Opening.ECO
				child.OpeningInherited = false
			}
			// Evaluate every retained move, including opponent continuations. Previously
			// only player candidates were evaluated, leaving most opponent nodes blank.
			if child.EvalCP == nil && g.c.EngineMode != "practical" {
				ev, evErr := g.eval(ctx, after.FEN())
				if ctx.Err() != nil {
					return r, ctx.Err()
				}
				if evErr != nil {
					child.EvalError = evErr.Error()
					g.warn("Some positions have no score: Cloud and/or local engine failed; see the position inspector.")
				} else {
					child.EvalCP = ev.cp
					child.EvalMate = ev.mate
					child.EvalSource = ev.src
				}
			}
			// BFS means the first occurrence is at the shallowest depth.
			// A repeated FEN is represented as an edge to its canonical node.
			// Its move, statistics and evaluation remain visible/exportable.
			posKey := after.Key()
			if canonical, exists := seen[posKey]; exists {
				child.Transposition = true
				child.TranspositionTo = &canonical
				child.Reason = fmt.Sprintf("Transposition to node #%d (already scheduled or explored)", canonical)
				g.transpositions++
				// No second expansion, and no duplicated Lichess or Stockfish work.
			} else {
				seen[posKey] = child.ID
				queue = append(queue, task{id: child.ID, board: after})
			}
		}
		if r.Truncated {
			break
		}
	}
	classifyTraps(r)
	g.emitLive(r, processed, len(queue), max(0, len(r.Nodes)-1), true)
	for warning := range g.warnings {
		r.Warnings = append(r.Warnings, warning)
	}
	sort.Strings(r.Warnings)
	if r.Truncated {
		r.Warnings = append(r.Warnings, "Generation stopped at the configured maximum number of nodes.")
	}
	r.Finished = time.Now().UTC().Format(time.RFC3339)
	if g.onProgress != nil {
		g.onProgress(len(r.Nodes), g.requests, "Generation finished")
	}
	return r, nil
}

// This intentionally conservative *heuristic* calls a move a potential practical trap
// only if an opponent reply is both played in >=5% of the relevant sample and worsens
// the opponent's position by >=1.50 pawns. It is not a tactical proof.
func classifyTraps(r *Report) {
	if r == nil || len(r.Nodes) == 0 {
		return
	}
	for _, n := range r.Nodes {
		n.TrapStatus = "no"
		if n.ID == 0 {
			n.TrapStatus = "n/a"
			continue
		}
		if n.EvalCP == nil {
			n.TrapStatus = "unverified"
		}
	}
	for _, n := range r.Nodes[1:] {
		if n.Parent < 0 {
			continue
		}
		parent := r.Nodes[n.Parent]
		// Did an opponent response worsen that opponent's evaluation?
		before, err := ParseFEN(parent.FEN)
		if err != nil || isPlayerTurn(before, r.Config) {
			continue
		}
		if n.EvalCP == nil || parent.EvalCP == nil {
			n.TrapStatus = "unverified"
			continue
		}
		swing := *n.EvalCP - *parent.EvalCP
		if r.Config.Color == "black" {
			swing = -swing
		}
		if swing >= 150 && n.Frequency >= 5 && n.Games >= r.Config.MinGames {
			n.TrapStatus = "yes"
			n.TrapReason = fmt.Sprintf("Opponent reply loses %.2f pawns relative to previous position; %.1f%% frequency, %d games (heuristic).", float64(swing)/100, n.Frequency, n.Games)
		}
	}
	// The preceding repertoire move is a trap opportunity if any retained opponent
	// response meets the swing criterion. Incomplete evaluations remain unverified.
	for _, n := range r.Nodes[1:] {
		p, err := ParseFEN(r.Nodes[n.Parent].FEN)
		if err != nil || !isPlayerTurn(p, r.Config) {
			continue
		}
		unknown := false
		for _, id := range n.Children {
			child := r.Nodes[id]
			if child.TrapStatus == "yes" {
				n.TrapStatus = "yes"
				n.TrapReason = "An opponent continuation is a potential trap: " + child.SAN
				break
			}
			if child.TrapStatus == "unverified" {
				unknown = true
			}
		}
		if n.TrapStatus != "yes" && (unknown || n.EvalCP == nil || len(n.Children) == 0) {
			n.TrapStatus = "unverified"
		}
	}
}
