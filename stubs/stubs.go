package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var GolHandler = "GolOperations.CalculateCellFlipped"
var Subscribe = "Broker.Subscribe"
var BrokerCalculate = "Broker.CalculateNextState"
var SDLSender = "DistributorOperations.SendToSdl"
var CellFLippedHandler = "Broker.RequestCellFlipped"
var RefreshHandler = "Broker.RefreshChan"

type Response struct {
	FlippedCell []util.Cell
	NewWorld    [][]uint8
	SliceNumber int
}

type Request struct {
	Key         string
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
	Key         string
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
	Turn        int
}
type CellFlippedRequest struct {
	CloseFlag bool
	Key       string
}
