package gol

import (
	"fmt"
	"github.com/ChrisGora/semaphore"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

type DistributorOperations struct{}

var readMutexSemaphore semaphore.Semaphore
var renderingSemaphore semaphore.Semaphore
var turn int
var channels distributorChannels
var turnComplete chan bool
var world [][]uint8

// a function that create an empty world

func MakeNewWorld(height, width int) [][]uint8 {
	newWorld := make([][]uint8, height)
	for i := range newWorld {
		newWorld[i] = make([]uint8, width)
	}
	return newWorld
}

func timer(p Params, currentState *[][]uint8, turns *int, eventChan chan<- Event, isEventChannelClosed *bool) {
	for {
		time.Sleep(2 * time.Second)

		if !*isEventChannelClosed {
			readMutexSemaphore.Wait()
			//realReadMutex.Lock()
			number := len(calculateAliveCells(p, *currentState))
			//realReadMutex.Unlock()
			readMutexSemaphore.Post()
			eventChan <- AliveCellsCount{CellsCount: number, CompletedTurns: *turns}

		} else {
			return
		}
	}
}

// calculate all alive cells.
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var list []util.Cell

	for n := 0; n < p.ImageHeight; n++ {
		for i := 0; i < p.ImageWidth; i++ {
			if world[n][i] == byte(255) {
				list = append(list, util.Cell{X: i, Y: n})
			}
		}
	}
	return list
}

func saveFile(c distributorChannels, p Params, world [][]uint8, turn int) {

	//fmt.Println(p)

	readMutexSemaphore.Wait()
	//realReadMutex.Lock()
	c.ioCommand <- ioOutput
	outputFilename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(turn)
	c.ioFilename <- outputFilename

	if len(world) == 0 {
		return
	}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	//realReadMutex.Unlock()
	readMutexSemaphore.Post()
	//fmt.Println(world)

}

func checkKeyPresses(p Params, c distributorChannels, world [][]uint8, turn *int, isEventChannelClosed *bool, listener *net.Listener, client *rpc.Client) {

	for {
		//fmt.Println("sas")
		key := <-c.keyPresses
		switch key {
		case 's':
			saveFile(c, p, world, *turn)
		case 'q':
			{

				saveFile(c, p, world, *turn)
				quitProgram(c, p, *listener, isEventChannelClosed)
			}
		case 'p':
			renderingSemaphore.Wait()
			c.events <- StateChange{*turn, Paused}
			for {
				key := <-c.keyPresses
				if key == 'p' {
					break
				}
			}
			renderingSemaphore.Post()
			c.events <- StateChange{*turn, Executing}
		case 'k':
			readMutexSemaphore.Wait()
			res := new(stubs.Response)
			client.Go(stubs.BrokerShutDownHandler, stubs.Request{}, res, nil)
			readMutexSemaphore.Post()
			saveFile(c, p, world, *turn)
			quitProgram(c, p, *listener, isEventChannelClosed)
			close(c.events)
			os.Exit(2)
		}

	}
}

func (d *DistributorOperations) SendToSdl(req stubs.SDLRequest, res *stubs.StatusReport) (err error) {
	flipped := req.FlippedCell

	renderingSemaphore.Wait()
	readMutexSemaphore.Wait()

	//a parallel way to calculate all cells flipped
	turn = req.Turn
	for _, cell := range flipped {
		channels.events <- CellFlipped{
			CompletedTurns: turn,
			Cell:           cell,
		}
		if world[cell.Y][cell.X] == 255 {
			world[cell.Y][cell.X] = 0
		} else if world[cell.Y][cell.X] == 0 {
			world[cell.Y][cell.X] = 255
		}
	}

	channels.events <- TurnComplete{CompletedTurns: turn}
	readMutexSemaphore.Post()
	renderingSemaphore.Post()
	turnComplete <- true
	return
}

// completely made for tests since tests do not run the main thus I can not obtain those data by flags
func ipGenerator(p Params) (string, string, string) {
	var broker string
	var localIP string
	var localPort string
	if p.Broker == "" {
		broker = "127.0.0.1:8030"
	} else {
		broker = p.Broker
	}

	if p.LocalIP == "" {
		localIP = "127.0.0.1"
	} else {
		localIP = p.LocalIP
	}

	if p.LocalPort == "" {
		localPort = "8080"
	} else {
		localPort = p.LocalPort
	}

	return broker, localIP, localPort
}

func initializeWorld(c distributorChannels, p Params) [][]uint8 {
	// let io start input
	c.ioCommand <- ioInput
	// HANBIN: send to the io goroutine the file name specified by the width and height
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
	c.ioFilename <- filename

	// TODO: Create a 2D slice to store the world.
	world = MakeNewWorld(p.ImageHeight, p.ImageWidth)
	for h := 0; h < p.ImageHeight; h++ {
		for w := 0; w < p.ImageWidth; w++ {
			world[h][w] = <-c.ioInput
		}
	}
	return world
}

func quitProgram(c distributorChannels, p Params, listener net.Listener, isEventChannelClosed *bool) {
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	*isEventChannelClosed = true

	err := listener.Close()
	if err != nil {
		return
	}
	close(c.events)
}

func runThroughTurns(client *rpc.Client, p Params, IP, port string, listener net.Listener) {
	if p.Turns > 0 {

		callBackIP := IP + ":" + port
		// iterate through the turns
		req := stubs.BrokerRequest{
			Turns:       p.Turns,
			Threads:     p.Threads,
			ImageWidth:  p.ImageWidth,
			ImageHeight: p.ImageHeight,
			World:       world,
			CallBackIP:  callBackIP,
		}

		cDone := make(chan *rpc.Call, 1)
		_ = client.Go(stubs.BrokerCalculate, req, nil, cDone)

		go rpc.Accept(listener)

		for i := 1; i <= p.Turns; i++ {
			<-turnComplete
		}

		<-cDone
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	broker, IP, port := ipGenerator(p)

	listener, _ := net.Listen("tcp", ":"+port)
	_ = rpc.Register(&DistributorOperations{})
	readMutexSemaphore = semaphore.Init(1, 1)
	renderingSemaphore = semaphore.Init(1, 1)
	turnComplete = make(chan bool)
	channels = c

	isEventChannelClosed := false

	world = initializeWorld(c, p)

	turn = 0

	for _, cell := range calculateAliveCells(p, world) {
		c.events <- CellFlipped{
			CompletedTurns: turn,
			Cell:           cell,
		}
	}

	// set the timer
	go timer(p, &world, &turn, c.events, &isEventChannelClosed)

	client, err := rpc.Dial("tcp", broker)
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			//fmt.Println(err)
			os.Exit(2)
		}
	}(client)

	if err != nil {
		fmt.Println("dial error:", err)
		os.Exit(2)
	}

	go checkKeyPresses(p, c, world, &turn, &isEventChannelClosed, &listener, client)

	// TODO: Execute all turns of the Game of Life.

	runThroughTurns(client, p, IP, port, listener)

	saveFile(c, p, world, p.Turns)

	quitProgram(c, p, listener, &isEventChannelClosed)

}
