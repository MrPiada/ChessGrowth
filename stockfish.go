package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func findStockfish(path string) (string, error) {
	if path != "" {
		s, e := os.Stat(path)
		if e != nil {
			return "", fmt.Errorf("Stockfish not found at %s", path)
		}
		if s.IsDir() {
			return "", fmt.Errorf("Stockfish path is a directory")
		}
		return path, nil
	}
	if p, e := exec.LookPath("stockfish"); e == nil {
		return p, nil
	}
	guesses := []string{"/opt/homebrew/bin/stockfish", "/usr/local/bin/stockfish", "/usr/bin/stockfish"}
	if runtime.GOOS == "windows" {
		guesses = append(guesses, `C:\Program Files\Stockfish\stockfish.exe`)
	}
	for _, p := range guesses {
		if s, e := os.Stat(p); e == nil && !s.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("Stockfish not installed. Choose Cloud evaluation or install Stockfish and select its executable.")
}

var scoreRE = regexp.MustCompile(`\bscore (cp|mate) (-?\d+)`)

func stockfishEval(ctx context.Context, path, fen string, depth int) (int, error) {
	cp, _, err := stockfishEvalDetailed(ctx, path, fen, depth)
	return cp, err
}

// Preserve the mate distance for the standard PGN [%eval #N] directive.
func stockfishEvalDetailed(ctx context.Context, path, fen string, depth int) (int, *int, error) {
	executable, err := findStockfish(path)
	if err != nil {
		return 0, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable)
	hideEngineConsole(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return 0, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, nil, err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return 0, nil, err
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	lines := make(chan string, 128)
	go func() {
		s := bufio.NewScanner(stdout)
		s.Buffer(make([]byte, 4096), 1<<20)
		for s.Scan() {
			select {
			case lines <- s.Text():
			case <-ctx.Done():
				return
			}
		}
		close(lines)
	}()
	send := func(s string) error { _, e := io.WriteString(stdin, s+"\n"); return e }
	readUntil := func(want string) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case l, ok := <-lines:
				if !ok {
					return errors.New("Stockfish exited unexpectedly")
				}
				if strings.HasPrefix(l, want) {
					return nil
				}
			}
		}
	}
	if err := send("uci"); err != nil {
		return 0, nil, err
	}
	if err := readUntil("uciok"); err != nil {
		return 0, nil, err
	}
	if err := send("isready"); err != nil {
		return 0, nil, err
	}
	if err := readUntil("readyok"); err != nil {
		return 0, nil, err
	}
	if err := send("position fen " + fen); err != nil {
		return 0, nil, err
	}
	if err := send("go depth " + strconv.Itoa(depth)); err != nil {
		return 0, nil, err
	}
	b, err := ParseFEN(fen)
	if err != nil {
		return 0, nil, err
	}
	last := 0
	var lastMate *int
	have := false
	for {
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case l, ok := <-lines:
			if !ok {
				return 0, nil, errors.New("Stockfish exited without bestmove")
			}
			if strings.HasPrefix(l, "info ") {
				x := scoreRE.FindStringSubmatch(l)
				if len(x) > 0 {
					v, _ := strconv.Atoi(x[2])
					if !b.White {
						v = -v
					}
					lastMate = nil
					if x[1] == "mate" {
						m := v
						lastMate = &m
						if v > 0 {
							last = 99999
						} else {
							last = -99999
						}
					} else {
						last = v
					}
					have = true
				}
			}
			if strings.HasPrefix(l, "bestmove ") {
				if !have {
					return 0, nil, fmt.Errorf("Stockfish returned no score")
				}
				return last, lastMate, nil
			}
		}
	}
}
