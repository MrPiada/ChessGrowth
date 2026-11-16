package main

import (
	"os"
	"strings"
	"testing"
)

func TestMaxNodesDefaultAndValidation(t *testing.T) {
	if got := defaults().MaxNodes; got != 10000 {
		t.Fatalf("unexpected MaxNodes default: %d", got)
	}
	for _, tc := range []struct {
		nodes int
		ok    bool
	}{
		{2, true}, {5000, true}, {10000, true}, {100000, true},
		{1, false}, {100001, false},
	} {
		c := defaults()
		c.MaxNodes = tc.nodes
		err := c.validate()
		if (err == nil) != tc.ok {
			t.Errorf("MaxNodes %d: validation error %v; want valid=%v", tc.nodes, err, tc.ok)
		}
	}
}

func TestMaxNodesBrowserLimits(t *testing.T) {
	html, err := os.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `id="max-nodes" value="10000" min="2" max="100000"`) {
		t.Fatal("browser input MaxNodes does not match backend limits")
	}
	js, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "nodeBudget:10000") {
		t.Fatal("progress UI does not use new default budget")
	}
}
