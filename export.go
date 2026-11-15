package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func quotePGN(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\"", "\\\"")
}
func movePrefix(b Board) string {
	if b.White {
		return fmt.Sprintf("%d.", b.Full)
	}
	return fmt.Sprintf("%d...", b.Full)
}
func (r *Report) ExportPGN() string {
	if r == nil || len(r.Nodes) == 0 {
		return ""
	}
	root, e := ParseFEN(r.Nodes[0].FEN)
	if e != nil {
		return ""
	}
	var o strings.Builder
	fmt.Fprintf(&o, "[Event \"%s\"]\n[Site \"ChessGrowth\"]\n[Date \"%s\"]\n[Round \"-\"]\n[White \"?\"]\n[Black \"?\"]\n[Result \"*\"]\n", quotePGN(r.Config.Event), time.Now().Format("2006.01.02"))
	fmt.Fprintf(&o, "[RepertoireColor \"%s\"]\n[LichessSpeedFilters \"%s\"]\n[LichessRatingFilters \"%s\"]\n", r.Config.Color, quotePGN(strings.Join(r.Config.Speeds, ",")), quotePGN(ratingList(r.Config.Ratings)))
	if root.FEN() != initialFEN {
		fmt.Fprintf(&o, "[SetUp \"1\"]\n[FEN \"%s\"]\n", quotePGN(root.FEN()))
	}
	o.WriteString("\n")
	live := []int{}
	for _, id := range r.Nodes[0].Children {
		if !r.Nodes[id].Deleted {
			live = append(live, id)
		}
	}
	if len(live) > 0 {
		writeMoveThenSiblings(r, live[0], live[1:], &o)
	}
	o.WriteString("*\n")
	return wrapPGN(o.String(), 110)
}
func ratingList(rs []int) string {
	s := []string{}
	for _, r := range rs {
		s = append(s, strconv.Itoa(r))
	}
	return strings.Join(s, ",")
}
func moveComment(n *Node) string {
	parts := []string{}
	if n.EvalMate != nil {
		parts = append(parts, fmt.Sprintf("[%%eval #%d]", *n.EvalMate))
	} else if n.EvalCP != nil {
		if *n.EvalCP > 90000 || *n.EvalCP < -90000 {
			parts = append(parts, "Eval: forced mate, distance unknown")
		} else {
			parts = append(parts, fmt.Sprintf("[%%eval %.2f]", float64(*n.EvalCP)/100))
		}
	} else {
		parts = append(parts, "Eval: unavailable")
	}
	if n.EvalSource != "" {
		parts = append(parts, "Source: "+n.EvalSource)
	}
	games := n.White + n.Draws + n.Black
	if games > 0 {
		parts = append(parts, fmt.Sprintf("Lichess W/D/B: %.1f%% / %.1f%% / %.1f%% (n=%d)", 100*float64(n.White)/float64(games), 100*float64(n.Draws)/float64(games), 100*float64(n.Black)/float64(games), games))
	} else {
		parts = append(parts, "Lichess W/D/B: not available")
	}
	parts = append(parts, fmt.Sprintf("Move frequency: %.1f%%", n.Frequency))
	if n.OpeningName != "" {
		parts = append(parts, "Opening: "+n.OpeningECO+" "+n.OpeningName)
	}
	trap := n.TrapStatus
	if trap == "" {
		trap = "unverified"
	}
	parts = append(parts, "Trap: "+trap)
	if n.Transposition {
		if n.TranspositionTo != nil {
			parts = append(parts, fmt.Sprintf("Transposition: same position as node #%d; continuation explored there", *n.TranspositionTo))
		} else {
			parts = append(parts, "Transposition")
		}
	}
	return "{ " + strings.Join(parts, "; ") + " } "
}
func writeLinear(r *Report, id int, o *strings.Builder) {
	if id < 1 || id >= len(r.Nodes) || r.Nodes[id].Deleted {
		return
	}
	n := r.Nodes[id]
	p := r.Nodes[n.Parent]
	b, e := ParseFEN(p.FEN)
	if e != nil {
		return
	}
	fmt.Fprintf(o, "%s %s %s", movePrefix(b), n.SAN, moveComment(n))
	children := liveChildren(r, n)
	if len(children) > 0 {
		writeMoveThenSiblings(r, children[0], children[1:], o)
	}
}
func liveChildren(r *Report, n *Node) []int {
	ids := []int{}
	for _, id := range n.Children {
		if !r.Nodes[id].Deleted {
			ids = append(ids, id)
		}
	}
	return ids
}
func writeMoveThenSiblings(r *Report, first int, siblings []int, o *strings.Builder) {
	if first < 1 || first >= len(r.Nodes) || r.Nodes[first].Deleted {
		return
	}
	n := r.Nodes[first]
	p := r.Nodes[n.Parent]
	b, e := ParseFEN(p.FEN)
	if e != nil {
		return
	}
	fmt.Fprintf(o, "%s %s %s", movePrefix(b), n.SAN, moveComment(n))
	for _, id := range siblings {
		o.WriteString("( ")
		writeLinear(r, id, o)
		o.WriteString(") ")
	}
	children := liveChildren(r, n)
	if len(children) > 0 {
		writeMoveThenSiblings(r, children[0], children[1:], o)
	}
}
func wrapPGN(s string, max int) string {
	i := strings.Index(s, "\n\n")
	if i < 0 {
		return s
	}
	headers, body := s[:i+2], s[i+2:]
	var out strings.Builder
	out.WriteString(headers)
	col := 0
	for _, tok := range strings.Fields(body) {
		if col+len(tok)+1 > max && col > 0 {
			out.WriteByte('\n')
			col = 0
		}
		if col > 0 {
			out.WriteByte(' ')
			col++
		}
		out.WriteString(tok)
		col += len(tok)
	}
	out.WriteByte('\n')
	return out.String()
}
func (r *Report) ExportJSON() []byte {
	data, _ := json.MarshalIndent(r, "", "  ")
	return bytes.TrimSpace(data)
}
