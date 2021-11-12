package main

import (
	"flag"
	"fmt"
	"github.com/ChrisGora/semaphore"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var (
	requestChannel  chan stubs.Request
	responseChannel chan stubs.Response
	nodeNumber      int
	nodeNumberMutex semaphore.Semaphore
)

func MakeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
}

func cutPartialSlice(startX, endX, startY, endY int, data func(y, x int) uint8) [][]uint8 {
	width := endX - startX
	height := endY - startY

	part := gol.MakeNewWorld(height, width)

	for h := startY; h < endY; h++ {
		for w := startX; w < endX; w++ {
			part[h-startY][w-startX] = data(h, w)
		}
	}

	return part
}

func Append2DSliceByColumn(twoDSlice [][][]uint8) [][]uint8 {
	newWorld := twoDSlice[0]
	for i := 1; i < len(twoDSlice); i++ {
		part := twoDSlice[i]
		for j := 0; j < len(newWorld); j++ {
			newWorld[j] = append(newWorld[j], part[j]...)
		}
	}

	return newWorld
}

//The subscriber loops run asynchronously, reading from the topic and sending err
//'job' pairs to their associated subscriber.
func subscriberLoop(client *rpc.Client, callback string) {
	for {
		request := <-requestChannel
		response := new(stubs.Response)
		err := client.Call(callback, request, response)
		if err != nil {
			fmt.Println("Error")
			fmt.Println(err)
			fmt.Println("Closing subscriber thread.")
			nodeNumberMutex.Wait()
			nodeNumber--
			nodeNumberMutex.Post()
			requestChannel <- request
			break
		}

		responseChannel <- *response

	}
}

//The subscribe function registers a worker to the topic, creating an RPC client,
//and will use the given callback string as the callback function whenever work
//is available.
func subscribe(nodeAddress string, callback string) (err error) {
	fmt.Println("Subscription request", nodeAddress)

	client, err := rpc.Dial("tcp", nodeAddress)
	if err == nil {
		go subscriberLoop(client, callback)
	} else {
		fmt.Println("Error subscribing ", nodeAddress)
		fmt.Println(err)
		return err
	}

	nodeNumberMutex.Wait()
	nodeNumber++
	nodeNumberMutex.Post()

	return
}

type Broker struct{}

func (b *Broker) Subscribe(req stubs.Subscription, res *stubs.StatusReport) (err error) {
	err = subscribe(req.NodeAddress, req.Callback)
	if err != nil {
		res.Message = "Error during subscription"
	}
	return err
}

func (b *Broker) CalculateNextState(req stubs.BrokerRequest, res *stubs.Response) (err error) {
	nodeNumberMutex.Wait()
	presentNodeNumber := nodeNumber
	nodeNumberMutex.Post()
	world := req.World
	data := MakeImmutableMatrix(world)

	if presentNodeNumber == 1 {
		nodeRequest := stubs.Request{Threads: req.Threads,
			SliceNumber: 0,
			StartY:      0,
			EndY:        req.ImageHeight,
			StartX:      0,
			EndX:        req.ImageWidth,
			World:       world}

		requestChannel <- nodeRequest

	} else {

		for n := 0; n < presentNodeNumber-1; n++ {
			startX := req.ImageWidth / presentNodeNumber * n
			endX := req.ImageWidth / presentNodeNumber * (n + 1)
			part := cutPartialSlice(startX, endX, 0, req.ImageHeight, data)
			nodeRequest := stubs.Request{Threads: req.Threads,
				SliceNumber: n,
				StartY:      0,
				EndY:        req.ImageHeight,
				StartX:      startX,
				EndX:        endX,
				World:       part}
			requestChannel <- nodeRequest
		}
		startX := req.ImageWidth / presentNodeNumber * (presentNodeNumber - 1)
		endX := req.ImageWidth
		part := cutPartialSlice(startX, endX, 0, req.ImageHeight, data)
		lastNodeRequest := stubs.Request{Threads: req.Threads,
			SliceNumber: presentNodeNumber - 1,
			StartY:      0,
			EndY:        req.ImageHeight,
			StartX:      startX,
			EndX:        endX,
			World:       part}

		requestChannel <- lastNodeRequest
	}

	var nextWorld [][]uint8
	var flipped []util.Cell
	SliceOf2DSlice := make([][][]uint8, presentNodeNumber)

	for n := 0; n < presentNodeNumber; n++ {

		response := <-responseChannel
		SliceOf2DSlice[response.SliceNumber] = response.NewWorld
		flipped = append(flipped, response.FlippedCell...)
	}

	//fmt.Println(SliceOf2DSlice)
	nextWorld = Append2DSliceByColumn(SliceOf2DSlice)

	res.NewWorld = nextWorld
	res.FlippedCell = flipped

	return err
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	_ = rpc.Register(&Broker{})
	requestChannel = make(chan stubs.Request)
	responseChannel = make(chan stubs.Response)
	nodeNumberMutex = semaphore.Init(1, 1)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(listener)
	fmt.Println("Port :listen on:", *pAddr)
	rpc.Accept(listener)
}
