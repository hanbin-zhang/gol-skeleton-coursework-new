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

//The subscriber loops run asynchronously, reading from the topic and sending the err
//'job' pairs to their associated subscriber.
func subscriber_loop(requestChan chan stubs.Request, client *rpc.Client, callback string) {
	for {
		request := <-requestChan
		response := new(stubs.Response)
		err := client.Call(callback, request, response)
		if err != nil {
			fmt.Println("Error")
			fmt.Println(err)
			fmt.Println("Closing subscriber thread.")
			nodeNumberMutex.Wait()
			nodeNumber--
			nodeNumberMutex.Post()
			requestChan <- request
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
		go subscriber_loop(requestChannel, client, callback)
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

func (b *Broker) CalculateNextState(req stubs.BrokerRequest, res stubs.Response) (err error) {

	nodeNumberMutex.Wait()
	presentNodeNumber := nodeNumber
	nodeNumberMutex.Post()

	world := req.World

	for n := 0; n < presentNodeNumber-1; n++ {
		nodeRequest := stubs.Request{Threads: req.Threads,
			SliceNumber: n,
			ImageWidth:  req.ImageWidth,
			ImageHeight: req.ImageHeight,
			StartY:      0,
			EndY:        req.ImageHeight,
			StartX:      req.ImageWidth / presentNodeNumber * n,
			EndX:        req.ImageWidth / presentNodeNumber * (n + 1),
			World:       world}
		requestChannel <- nodeRequest
	}

	lastNodeRequest := stubs.Request{Threads: req.Threads,
		SliceNumber: presentNodeNumber - 1,
		ImageWidth:  req.ImageWidth,
		ImageHeight: req.ImageHeight,
		StartY:      0,
		EndY:        req.ImageHeight,
		StartX:      req.ImageWidth/presentNodeNumber*presentNodeNumber - 1,
		EndX:        req.ImageWidth,
		World:       world}
	requestChannel <- lastNodeRequest

	var nextWorld [][]uint8
	var flipped []util.Cell
	SliceOf2DSlice := make([][][]uint8, presentNodeNumber)

	for n := 0; n < presentNodeNumber; n++ {
		response := <-responseChannel

		SliceOf2DSlice[response.SliceNumber] = response.NewWorld
		flipped = append(flipped, response.FlippedCell...)
	}

	nextWorld = gol.Append2DSliceByColumn(SliceOf2DSlice)

	res.NewWorld = nextWorld
	res.FlippedCell = flipped

	return err
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Broker{})
	nodeNumberMutex = semaphore.Init(1, 1)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	fmt.Println("Port :listen on:", *pAddr)
	rpc.Accept(listener)
}
