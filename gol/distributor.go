package gol

import (
	"fmt"
	"github.com/ChrisGora/semaphore"
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

func timer(p Params, eventChan chan<- Event, isEventChannelClosed *bool) {
	for {
		time.Sleep(2 * time.Second)

		if !*isEventChannelClosed {
			readMutexSemaphore.Wait()
			//realReadMutex.Lock()
			number := len(calculateAliveCells(p, world))
			//realReadMutex.Unlock()
			readMutexSemaphore.Post()
			eventChan <- AliveCellsCount{CellsCount: number, CompletedTurns: turn}

		} else {
			return
		}
	}
}

// calculate all alive cells.
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var list []util.Cell

	for n := 0; n < len(world); n++ {
		for i := 0; i < len(world[0]); i++ {
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

func checkKeyPresses(p Params, c distributorChannels, world [][]uint8, turn *int, isEventChannelClosed *bool, client *rpc.Client) {

	for {
		//fmt.Println("sas")
		key := <-c.keyPresses
		switch key {
		case 's':
			saveFile(c, p, world, *turn)
		case 'q':
			{
				client.Call(stubs.CellFLippedHandler, stubs.CellFlippedRequest{CloseFlag: false}, nil)
				saveFile(c, p, world, *turn)
				c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}

				// Make sure that the Io has finished any output before exiting.
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle

				c.events <- StateChange{*turn, Quitting}

				// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
				*isEventChannelClosed = true
				//l := *plistener

				close(c.events)
				os.Exit(2)
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
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) { /*
		for  {
			listener, errL := net.Listen("tcp", "127.0.0.1:8080")
			fmt.Println(errL)
			if errL==nil {
				listener.Close()
				break
			}
		}*/
	//fmt.Println(errL)
	//_ = rpc.Register(&DistributorOperations{})
	readMutexSemaphore = semaphore.Init(1, 1)
	renderingSemaphore = semaphore.Init(1, 1)
	turnComplete = make(chan bool)
	channels = c

	isEventChannelClosed := false
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

	turn = 0

	//fmt.Println(calculateAliveCells(p, world))
	for _, cell := range calculateAliveCells(p, world) {
		c.events <- CellFlipped{
			CompletedTurns: turn,
			Cell:           cell,
		}
	}
	//c.events <- TurnComplete{CompletedTurns: turn}

	// set the timer
	go timer(p, c.events, &isEventChannelClosed)

	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	}(client)
	key := time.Now().String()
	fmt.Println(key)
	client.Call(stubs.RefreshHandler, stubs.Request{Key: key}, nil)

	go checkKeyPresses(p, c, world, &turn, &isEventChannelClosed, client)

	// TODO: Execute all turns of the Game of Life.

	if p.Turns > 0 {

		callBackIP := "127.0.0.1:8080"
		// iterate through the turns
		req := stubs.BrokerRequest{
			Turns:       p.Turns,
			Threads:     p.Threads,
			ImageWidth:  p.ImageWidth,
			ImageHeight: p.ImageHeight,
			World:       world,
			CallBackIP:  callBackIP,
			Key:         key,
		}

		//res := new(stubs.Response)
		cDone := make(chan *rpc.Call, 1)
		_ = client.Go(stubs.BrokerCalculate, req, nil, cDone)

		for i := 1; i <= p.Turns; i++ {
			stubsReq := stubs.CellFlippedRequest{CloseFlag: true, Key: key}
			res := new(stubs.SDLRequest)

			client.Call(stubs.CellFLippedHandler, stubsReq, res)

			flipped := res.FlippedCell

			renderingSemaphore.Wait()
			readMutexSemaphore.Wait()
			//a parallel way to calculate all cells flipped
			//fmt.Println(flipped)
			turn = res.Turn
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
		}

	}

	saveFile(c, p, world, p.Turns)

	// TODO: output proceeded map IO
	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	isEventChannelClosed = true
	close(c.events)
	//os.Exit(2)
}
