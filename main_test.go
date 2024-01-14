package main

import (
	"testing"

	"github.com/test-go/testify/require"
)

func TestPegNeighbor(t *testing.T) {
	t.Parallel()

	g := newGame(6, 7)
	g.reset()
	p1 := g.addPeg(0)
	p2 := g.addPeg(0)
	got := p1.neighbor(North)
	require.Equal(t, p2, got)
}
