package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var GolHandler = "GolOperations.CalculateCellFlipped"
var Subscribe = "Broker.Subscribe"
var BrokerCalculate = "Broker.CalculateNextState"
var SDLSender = "DistributorOperations.SendToSdl"

type Response struct {
	FlippedCell []util.Cell
	NewWorld    [][]uint8
	SliceNumber int
}

type Request struct {
	Threads     int
	SliceNumber int
	ImageWidth  int
	ImageHeight int
	StartY      int
	EndY        int
	StartX      int
	EndX        int
	World       [][]uint8
}

type BrokerRequest struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	World       [][]uint8
	CallBackIP  string
}

type Subscription struct {
	NodeAddress string
	Callback    string
}

type StatusReport struct {
	Message string
}

type SDLRequest struct {
	FlippedCell []util.Cell
}
