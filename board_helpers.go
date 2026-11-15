package main

import (
	"fmt"
	"strings"
)

func moveUCI(m Move) string {
	s := sq(m.From) + sq(m.To)
	if m.Promo != 0 {
		s += string(m.Promo)
	}
	return s
}
func (b Board) ParseUCI(u string) (Move, error) {
	u = strings.ToLower(strings.TrimSpace(u))
	for _, m := range b.moves() {
		if moveUCI(m) == u {
			return m, nil
		}
	}
	return Move{}, fmt.Errorf("illegal UCI move %q in %s", u, b.FEN())
}
func (b Board) ParseMove(s string) (Move, error) {
	if len(s) == 4 || len(s) == 5 {
		if m, e := b.ParseUCI(s); e == nil {
			return m, nil
		}
	}
	return b.ParseSAN(s)
}
func (b Board) Ply() int {
	p := (b.Full - 1) * 2
	if !b.White {
		p++
	}
	return p
}
func validBoard(b Board) error {
	// The built-in chess logic supports standard chess only.
	// Reject nonsensical turn/check combinations to avoid illegal continuation.
	if b.check(!b.White) {
		return fmt.Errorf("invalid position: the side that just moved is in check")
	}
	return nil
}
