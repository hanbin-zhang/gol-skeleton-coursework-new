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

var readMutexSemaphore semaphore.Semaphore
var renderingSemaphore semaphore.Semaphore
var turn int

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

func checkKeyPresses(p Params, c distributorChannels, world [][]uint8, turn *int, isEventChannelClosed *bool) {

	for {
		//fmt.Println("sas")
		key := <-c.keyPresses
		switch key {
		case 's':
			saveFile(c, p, world, *turn)
		case 'q':
			{

				saveFile(c, p, world, *turn)
				c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}

				// Make sure that the Io has finished any output before exiting.
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle

				c.events <- StateChange{*turn, Quitting}

				// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
				*isEventChannelClosed = true
				close(c.events)
				//os.Exit(2)
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
func distributor(p Params, c distributorChannels) {

	readMutexSemaphore = semaphore.Init(1, 1)
	renderingSemaphore = semaphore.Init(1, 1)

	isEventChannelClosed := false
	// let io start input
	c.ioCommand <- ioInput
	// HANBIN: send to the io goroutine the file name specified by the width and height
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
	c.ioFilename <- filename

	// TODO: Create a 2D slice to store the world.
	world := MakeNewWorld(p.ImageHeight, p.ImageWidth)
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
	//go timer(p, &world, &turn, c.events, &isEventChannelClosed)

	go checkKeyPresses(p, c, world, &turn, &isEventChannelClosed)

	// TODO: Execute all turns of the Game of Life.

	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	}(client)

	// iterate through the turns
	req := stubs.BrokerRequest{
		Threads:     p.Threads,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		World:       world,
		Turns:       p.Turns,
	}

	res := new(stubs.Response)

	_ = client.Call(stubs.BrokerCalculate, req, res)

	world = res.NewWorld

	// HANBIN: sometimes, is just not too good to something too early
	//
	//
	//
	//fmt.Println("aaa")
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
