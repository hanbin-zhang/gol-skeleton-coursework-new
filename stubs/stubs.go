package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var GolHandler = "GolOperations.CalculateCellFlipped"
var Subscribe = "Broker.Subscribe"
var BrokerCalculate = "Broker.CalculateNextState"

type Response struct {
	FlippedCell []util.Cell
	NewWorld    [][]uint8
	sliceNumber int
}

type Request struct {
	sliceNumber int
	ImageWidth  int
	ImageHeight int
	StartY      int
	EndY        int
	StartX      int
	EndX        int
	World       [][]uint8
}

type BrokerRequest struct {
	Threads     int
	ImageWidth  int
	ImageHeight int
	World       [][]uint8
}

type Subscription struct {
	NodeAddress string
	Callback    string
}

type StatusReport struct {
	Message string
}
