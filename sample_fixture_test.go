package main

import (
	"time"
)

func sampleReportForTest() *Report {
	c := defaults()
	c.MaxDepth = 9
	c.Event = "SyntheticTest"
	r := newReport(c)
	start, _ := ParseFEN(initialFEN)
	r.add(-1, start, Move{}, "Synthetic start")
	lines := [][]string{
		{"e4", "e5", "Nf3", "Nc6", "Bb5", "a6", "Ba4", "Nf6"},
		{"e4", "e5", "Nf3", "Nc6", "Bb5", "Nf6", "O-O", "Nxe4"},
		{"e4", "c5", "Nf3", "d6", "d4", "cxd4", "Nxd4", "Nf6"},
		{"e4", "c5", "Nf3", "Nc6", "d4", "cxd4", "Nxd4", "e5"},
		{"e4", "e6", "d4", "d5", "Nc3", "Bb4", "e5", "c5"},
		{"d4", "d5", "c4", "e6", "Nc3", "Nf6", "Bg5", "Be7"},
		{"d4", "Nf6", "c4", "g6", "Nc3", "Bg7", "e4", "d6"},
		{"d4", "Nf6", "c4", "e6", "Nc3", "Bb4", "e3", "O-O"},
		{"c4", "e5", "Nc3", "Nf6", "Nf3", "Nc6", "g3", "d5"},
		{"Nf3", "d5", "g3", "Nf6", "Bg2", "e6", "O-O", "Be7"},
	}
	for _, line := range lines {
		cur := start
		parent := 0
		for ply, step := range line {
			m, e := cur.ParseSAN(step)
			if e != nil {
				break
			}
			u := moveUCI(m)
			child := -1
			for _, id := range r.Nodes[parent].Children {
				if r.Nodes[id].UCI == u {
					child = id
					break
				}
			}
			next := cur.Apply(m)
			if child < 0 {
				n := r.add(parent, next, m, "")
				child = n.ID
				n.Games = int64(1200/(1+ply) + 120/(1+child))
				n.Frequency = float64(80 / (1 + ply%3))
				n.White = n.Games / 2
				n.Draws = n.Games / 4
				n.Black = n.Games - n.White - n.Draws
				cp := []int{21, 14, 28, -5, 40, 18, 33, 3}[ply]
				n.EvalCP = &cp
				n.EvalSource = "Synthetic test fixture"
			}
			parent = child
			cur = next
		}
	}
	r.Warnings = []string{"Synthetic test fixture, not live Lichess data."}
	r.Finished = time.Now().UTC().Format(time.RFC3339)
	return r
}
