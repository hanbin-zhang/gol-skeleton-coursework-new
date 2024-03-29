package main

import (
	"flag"
	"fmt"
	"github.com/ChrisGora/semaphore"
	"net"
	"net/rpc"
	"os"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var (
	requestChannel       chan stubs.Request
	responseChannel      chan stubs.Response
	nodeNumber           int
	nodeNumberMutex      semaphore.Semaphore
	worldUpdateSemaphore semaphore.Semaphore
	ignitionSemaphore    semaphore.Semaphore
	//world                [][]uint8
	turn            int
	shutDownChannel chan bool
	clientMap       map[*rpc.Client]bool
)

//The subscriber loops run asynchronously, reading from the topic and sending err
//'job' pairs to their associated subscriber.
func subscriberLoop(client *rpc.Client, callback string) {
LOOP:
	for {
		select {
		case req := <-requestChannel:
			request := req
			response := new(stubs.Response)
			err := client.Call(callback, request, response)
			if err != nil {
				fmt.Println("Error")
				fmt.Println(err)
				fmt.Println("Closing subscriber thread.")
				nodeNumberMutex.Wait()
				nodeNumber--
				if nodeNumber == 0 {
					ignitionSemaphore.Wait()
				}
				delete(clientMap, client)
				nodeNumberMutex.Post()
				requestChannel <- request
				break LOOP
			}
			responseChannel <- *response
		case <-shutDownChannel:
			res := new(stubs.Response)
			client.Go(stubs.ServerShutDownHandler, stubs.Request{}, res, nil)
		}

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
	if ignitionSemaphore.GetValue() < 1 {
		ignitionSemaphore.Post()
	}
	clientMap[client] = true
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

func (b *Broker) ShutEveryThingDown(req stubs.Request, res *stubs.Response) (err error) {
	for key := range clientMap {
		res1 := new(stubs.Response)
		key.Go(stubs.ServerShutDownHandler, stubs.Request{}, res1, nil)
	}
	fmt.Println("Broker exiting...")
	os.Exit(2)
	return
}

func sendRequest(req stubs.BrokerRequest, presentNodeNumber int, world [][]uint8) {
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
		StartX:      req.ImageWidth / presentNodeNumber * (presentNodeNumber - 1),
		EndX:        req.ImageWidth,
		World:       world}

	requestChannel <- lastNodeRequest
}

func getNodeNumber() int {
	var presentNodeNumber int

	for {
		nodeNumberMutex.Wait()
		presentNodeNumber = nodeNumber
		nodeNumberMutex.Post()
		if nodeNumber == 0 {
			ignitionSemaphore.Wait()
		} else {
			break
		}
	}
	return presentNodeNumber
}

func (b *Broker) CalculateNextState(req stubs.BrokerRequest, res *stubs.Response) (err error) {
	world := req.World

	turn = 0
	client, _ := rpc.Dial("tcp", req.CallBackIP)

	for t := 1; t <= req.Turns; t++ {
		presentNodeNumber := getNodeNumber()

		sendRequest(req, presentNodeNumber, world)

		var flipped []util.Cell

		for n := 0; n < presentNodeNumber; n++ {
			response := <-responseChannel
			flipped = append(flipped, response.FlippedCell...)
		}

		SDLRequest := stubs.SDLRequest{FlippedCell: flipped, Turn: t}
		SDLRes := new(stubs.StatusReport)

		err = client.Call(stubs.SDLSender, SDLRequest, SDLRes)
		if err != nil {
			fmt.Println(err)
			_ = client.Close()
			break
		}
		worldUpdateSemaphore.Wait()
		turn++
		for _, cell := range flipped {
			if world[cell.Y][cell.X] == 255 {
				world[cell.Y][cell.X] = 0
			} else if world[cell.Y][cell.X] == 0 {
				world[cell.Y][cell.X] = 255
			}
		}
		worldUpdateSemaphore.Post()

	}
	_ = client.Close()
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	_ = rpc.Register(&Broker{})
	ignitionSemaphore = semaphore.Init(1, 0)
	clientMap = make(map[*rpc.Client]bool)
	requestChannel = make(chan stubs.Request, 8)
	responseChannel = make(chan stubs.Response)
	shutDownChannel = make(chan bool)
	nodeNumberMutex = semaphore.Init(1, 1)
	worldUpdateSemaphore = semaphore.Init(1, 1)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(listener)
	fmt.Println("Port :listen on:", stubs.GetOutboundIP().String(), ":", *pAddr)
	rpc.Accept(listener)
}
