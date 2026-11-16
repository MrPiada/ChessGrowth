package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/* assets/*
var content embed.FS
var version = "1.3.1"

type application struct {
	live                                           []LiveNode
	processed, pending, current                    int
	localEvaluations, cloudLookups, transpositions int
	mu                                             sync.Mutex
	phase                                          string
	message                                        string
	nodes, requests                                int
	lastError                                      string
	result                                         *Report
	undo                                           [][]int
	cancel                                         context.CancelFunc
	tokens                                         tokenStore
	lifecycle                                      *browserLifecycle
}

func newApplication() *application {
	return &application{phase: "idle", message: "Configure an opening to begin.", tokens: newTokenStore(), lifecycle: newBrowserLifecycle(false, 6*time.Second, nil)}
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	respond(w, map[string]any{"error": err.Error()})
}
func method(w http.ResponseWriter, r *http.Request, want string) bool {
	if r.Method != want {
		fail(w, 405, errors.New("method not allowed"))
		return false
	}
	return true
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		fail(w, 400, err)
		return false
	}
	return true
}
func credentialRequestAllowed(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		return true
	}
	if r.Method == http.MethodPost && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
			return false
		}
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	return true
}
func (a *application) handlers() http.Handler {
	mux := http.NewServeMux()
	webFS, e := fs.Sub(content, "web")
	if e != nil {
		panic(e)
	}
	assetFS, e := fs.Sub(content, "assets")
	if e != nil {
		panic(e)
	}
	mux.Handle("/api/lifecycle/", a.lifecycle)
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetFS))))
	mux.Handle("/", http.FileServer(http.FS(webFS)))
	mux.HandleFunc("/api/meta", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "GET") {
			return
		}
		stockfish, err := findStockfish("")
		respond(w, map[string]any{"version": version, "platform": runtime.GOOS, "stockfish": err == nil, "stockfish_path": stockfish, "default": defaults()})
	})
	mux.HandleFunc("/api/credentials", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !credentialRequestAllowed(r) {
			fail(w, 403, errors.New("invalid credential request origin or content type"))
			return
		}
		switch r.Method {
		case http.MethodGet:
			token, source, err := activeToken(a.tokens)
			if err != nil {
				fail(w, 500, err)
				return
			}
			respond(w, map[string]any{"saved": token != "", "source": source, "storage": a.tokens.Description(), "environment_override": strings.TrimSpace(os.Getenv("LICHESS_TOKEN")) != ""})
		case http.MethodPost:
			var input struct {
				Token string `json:"token"`
			}
			if !decode(w, r, &input) {
				return
			}
			if err := a.tokens.Save(strings.TrimSpace(input.Token)); err != nil {
				fail(w, 400, err)
				return
			}
			respond(w, map[string]any{"saved": true, "storage": a.tokens.Description()})
		case http.MethodDelete:
			if err := a.tokens.Delete(); err != nil {
				fail(w, 500, err)
				return
			}
			respond(w, map[string]any{"saved": false})
		default:
			fail(w, 405, errors.New("method not allowed"))
		}
	})
	mux.HandleFunc("/api/engine", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "GET") {
			return
		}
		path := r.URL.Query().Get("path")
		resolved, err := findStockfish(path)
		respond(w, map[string]any{"available": err == nil, "path": resolved, "error": func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}()})
	})
	mux.HandleFunc("/api/board", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "GET") {
			return
		}
		fen := r.URL.Query().Get("fen")
		if fen == "" {
			fen = initialFEN
		}
		b, err := ParseFEN(fen)
		if err != nil {
			fail(w, 400, err)
			return
		}
		if err = validBoard(b); err != nil {
			fail(w, 400, err)
			return
		}
		legal := []map[string]any{}
		for _, m := range b.moves() {
			legal = append(legal, map[string]any{"uci": moveUCI(m), "san": b.SAN(m), "from": sq(m.From), "to": sq(m.To)})
		}
		respond(w, map[string]any{"fen": b.FEN(), "legal": legal, "turn": func() string {
			if b.White {
				return "white"
			}
			return "black"
		}()})
	})
	mux.HandleFunc("/api/apply", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "POST") {
			return
		}
		var req struct {
			FEN string `json:"fen"`
			UCI string `json:"uci"`
		}
		if !decode(w, r, &req) {
			return
		}
		b, err := ParseFEN(req.FEN)
		if err != nil {
			fail(w, 400, err)
			return
		}
		m, err := b.ParseMove(req.UCI)
		if err != nil {
			fail(w, 400, err)
			return
		}
		respond(w, map[string]any{"fen": b.Apply(m).FEN(), "san": b.SAN(m), "uci": moveUCI(m)})
	})
	mux.HandleFunc("/api/sequence", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "POST") {
			return
		}
		var req struct {
			FEN   string `json:"fen"`
			Moves string `json:"moves"`
		}
		if !decode(w, r, &req) {
			return
		}
		if req.FEN == "" {
			req.FEN = initialFEN
		}
		b, err := ParseFEN(req.FEN)
		if err != nil {
			fail(w, 400, err)
			return
		}
		history := []map[string]string{}
		for _, tok := range tokenizeMoves(req.Moves) {
			m, e := b.ParseMove(tok)
			if e != nil {
				fail(w, 400, fmt.Errorf("move %q: %w", tok, e))
				return
			}
			history = append(history, map[string]string{"san": b.SAN(m), "uci": moveUCI(m)})
			b = b.Apply(m)
		}
		respond(w, map[string]any{"fen": b.FEN(), "moves": history})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "GET") {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		active, leaves := 0, 0
		if a.result != nil {
			active = a.result.activeCount()
			leaves = a.result.activeLeaves()
		}
		respond(w, map[string]any{"phase": a.phase, "message": a.message, "nodes": a.nodes, "requests": a.requests, "error": a.lastError, "active": active, "leaves": leaves, "ready": a.result != nil})
	})
	mux.HandleFunc("/api/progress", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "GET") {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		respond(w, map[string]any{"phase": a.phase, "nodes": a.nodes, "requests": a.requests, "processed": a.processed, "pending": a.pending, "current": a.current, "local_evaluations": a.localEvaluations, "cloud_lookups": a.cloudLookups, "transpositions": a.transpositions, "tree": a.live})
	})
	mux.HandleFunc("/api/start", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "POST") {
			return
		}
		var cfg Config
		if !decode(w, r, &cfg) {
			return
		}
		if e := cfg.validate(); e != nil {
			fail(w, 400, e)
			return
		}
		a.mu.Lock()
		if a.phase == "running" {
			a.mu.Unlock()
			fail(w, 409, errors.New("an analysis is already running"))
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		a.cancel = cancel
		a.phase = "running"
		a.message = "Loading your saved Lichess token (check macOS Keychain if prompted)"
		a.lastError = ""
		a.result = nil
		a.undo = nil
		a.nodes = 0
		a.requests = 0
		a.live = nil
		a.processed = 0
		a.pending = 0
		a.current = 0
		a.localEvaluations = 0
		a.cloudLookups = 0
		a.transpositions = 0
		a.mu.Unlock()
		go func() {
			g := newGenerator(cfg, func(nodes, requests int, msg string) {
				a.mu.Lock()
				a.nodes = nodes
				a.requests = requests
				a.message = msg
				a.mu.Unlock()
			})
			g.onLive = func(nodes []LiveNode, processed, pending, current int) {
				a.mu.Lock()
				defer a.mu.Unlock()
				a.live = nodes
				a.processed = processed
				a.pending = pending
				a.current = current
				a.localEvaluations = g.localCount
				a.cloudLookups = g.cloudCount
				a.transpositions = g.transpositions
			}
			report, err := g.run(ctx)
			a.mu.Lock()
			defer a.mu.Unlock()
			a.cancel = nil
			if report != nil && len(report.Nodes) > 0 {
				a.result = report
			}
			switch {
			case errors.Is(err, context.Canceled):
				a.phase = "cancelled"
				a.message = "Generation stopped. Partial tree available."
			case err != nil:
				a.phase = "error"
				a.message = "Generation stopped with an error"
				a.lastError = err.Error()
			default:
				a.phase = "done"
				a.message = "Repertoire ready for review and export"
			}
		}()
		respond(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/cancel", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "POST") {
			return
		}
		a.mu.Lock()
		cancel := a.cancel
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		respond(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/tree", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "GET") {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.result == nil {
			fail(w, 404, errors.New("no repertoire generated yet"))
			return
		}
		respond(w, a.result)
	})
	mux.HandleFunc("/api/prune", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "POST") {
			return
		}
		var req struct {
			ID int `json:"id"`
		}
		if !decode(w, r, &req) {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.phase == "running" {
			fail(w, 409, errors.New("finish or cancel generation first"))
			return
		}
		if a.result == nil {
			fail(w, 404, errors.New("no repertoire"))
			return
		}
		ids, err := a.result.prune(req.ID)
		if err != nil {
			fail(w, 400, err)
			return
		}
		a.undo = append(a.undo, ids)
		respond(w, map[string]any{"removed": len(ids), "active": a.result.activeCount(), "leaves": a.result.activeLeaves()})
	})
	mux.HandleFunc("/api/undo", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "POST") {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.result == nil || len(a.undo) == 0 {
			fail(w, 400, errors.New("nothing to undo"))
			return
		}
		ids := a.undo[len(a.undo)-1]
		a.undo = a.undo[:len(a.undo)-1]
		a.result.restore(ids)
		respond(w, map[string]any{"restored": len(ids)})
	})
	mux.HandleFunc("/api/export/pgn", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "GET") {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.result == nil {
			fail(w, 404, errors.New("no repertoire"))
			return
		}
		w.Header().Set("Content-Type", "application/x-chess-pgn; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="chessgrowth-repertoire.pgn"`)
		_, _ = w.Write([]byte(a.result.ExportPGN()))
	})
	mux.HandleFunc("/api/export/json", func(w http.ResponseWriter, r *http.Request) {
		if !method(w, r, "GET") {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.result == nil {
			fail(w, 404, errors.New("no repertoire"))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="chessgrowth-repertoire.json"`)
		_, _ = w.Write(a.result.ExportJSON())
	})
	return mux
}
func tokenizeMoves(s string) []string {
	// A lightweight mainline importer: strips PGN tags, brace comments, variations and move numbers.
	var clean strings.Builder
	inComment, inTag, variation := false, false, 0
	for _, ch := range s {
		switch {
		case ch == '{':
			inComment = true
			continue
		case ch == '}':
			inComment = false
			continue
		case ch == '[' && !inComment:
			inTag = true
			continue
		case ch == ']' && inTag:
			inTag = false
			continue
		case inComment || inTag:
			continue
		case ch == '(':
			variation++
			continue
		case ch == ')':
			if variation > 0 {
				variation--
			}
			continue
		case variation > 0:
			continue
		default:
			clean.WriteRune(ch)
		}
	}
	tokens := []string{}
	for _, t := range strings.Fields(clean.String()) {
		if strings.HasPrefix(t, "$") || t == "*" || t == "1-0" || t == "0-1" || t == "1/2-1/2" {
			continue
		}
		if n := strings.LastIndex(t, "."); n >= 0 {
			t = t[n+1:]
		}
		t = strings.TrimSpace(t)
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	return tokens
}
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
func main() {
	noOpen := flag.Bool("no-open", false, "do not launch the browser")
	addr := flag.String("listen", "127.0.0.1:0", "HTTP listener; defaults to local-only random port")
	showVersion := flag.Bool("version", false, "print ChessGrowth version")
	flag.Parse()
	if *showVersion {
		fmt.Println("ChessGrowth", version)
		return
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	app := newApplication()
	s := &http.Server{Handler: app.handlers(), ReadHeaderTimeout: 10 * time.Second}
	appCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	shutdown := sync.OnceFunc(func() {
		app.mu.Lock()
		if app.cancel != nil {
			app.cancel()
		}
		app.mu.Unlock()
		app.lifecycle.closeAll()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			_ = s.Close()
		}
	})
	app.lifecycle = newBrowserLifecycle(!*noOpen, 6*time.Second, func() {
		log.Println("Last ChessGrowth browser window closed; stopping local server")
		time.Sleep(100 * time.Millisecond)
		shutdown()
	})
	s.Handler = app.handlers()
	go app.lifecycle.watch(appCtx)
	go func() { <-appCtx.Done(); shutdown() }()
	url := "http://" + listener.Addr().String()
	fmt.Println("ChessGrowth", version, "is running at", url)
	if *noOpen {
		fmt.Println("Headless mode: close with Ctrl+C (browser auto-exit disabled)")
	} else {
		fmt.Println("Close the browser tab or click Quit application to exit.")
	}
	if !*noOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if appCtx.Err() == nil {
				openBrowser(url)
			}
		}()
	}
	if err = s.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
