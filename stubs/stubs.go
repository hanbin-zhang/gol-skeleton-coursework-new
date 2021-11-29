package stubs

import (
	"log"
	"net"
	"uk.ac.bris.cs/gameoflife/util"
)

var GolHandler = "GolOperations.CalculateCellFlipped"
var Subscribe = "Broker.Subscribe"
var BrokerCalculate = "Broker.CalculateNextState"
var SDLSender = "DistributorOperations.SendToSdl"
var ServerShutDownHandler = "GolOperations.ShutDownServer"
var BrokerShutDownHandler = "Broker.ShutEveryThingDown"

type Response struct {
	FlippedCell []util.Cell
	NewWorld    [][]uint8
	SliceNumber int
}

func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
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
	Turn        int
}
