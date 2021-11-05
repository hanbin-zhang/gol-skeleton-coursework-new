package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var GolHandler = "GolOperations.CalculateCellFlipped"

type Response struct {
	FlippedCell []util.Cell
	NewWorld    [][]uint8
}

type Request struct {
	Threads     int
	ImageWidth  int
	ImageHeight int
	World       [][]uint8
}
