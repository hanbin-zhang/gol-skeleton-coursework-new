package stubs

import (
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

var GolHandler = "GolOperations.CalculateCellFlipped"

type Response struct {
	FlippedCell []util.Cell
}

type Request struct {
	P     gol.Params
	World [][]uint8
}
