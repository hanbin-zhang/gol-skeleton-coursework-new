package main

import (
	"flag"
	"fmt"
	"github.com/ChrisGora/semaphore"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
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
	fmt.Println("Subscription request")

	nodeNumberMutex.Wait()
	nodeNumber++
	nodeNumberMutex.Post()

	client, err := rpc.Dial("tcp", nodeAddress)
	if err == nil {
		go subscriber_loop(requestChannel, client, callback)
	} else {
		fmt.Println("Error subscribing ", nodeAddress)
		fmt.Println(err)
		return err
	}
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

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Broker{})
	nodeNumberMutex = semaphore.Init(1, 1)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
