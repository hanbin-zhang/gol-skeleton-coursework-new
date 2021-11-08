package gol

import (
	"github.com/ChrisGora/semaphore"
	"net/rpc"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type workerChannels struct {
	worldSlice  chan [][]uint8
	flippedCell chan []util.Cell
}

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

//var realReadMutex sync.Mutex

func rpcWorker(ip string, p Params, world [][]uint8, serverID, serverNumber int, c chan stubs.Response) {

	startY := 0
	endY := 0

	if serverID != serverNumber-1 {
		startY = p.ImageHeight / serverNumber * serverID
		endY = p.ImageHeight / serverNumber * serverID
	} else {
		startY = p.ImageHeight / serverNumber * serverID
		endY = p.ImageHeight
	}

	request := stubs.Request{Threads: p.Threads,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		StartX:      0,
		EndX:        p.ImageWidth,
		StartY:      startY,
		EndY:        endY,
		World:       world}

	response := new(stubs.Response)

	client, _ := rpc.Dial("tcp", ip)

	client.Call(stubs.GolHandler, request, &response)

	c <- *response

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

func MakeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
}

// a function that create an empty world

func MakeNewWorld(height, width int) [][]uint8 {
	newWorld := make([][]uint8, height)
	for i := range newWorld {
		newWorld[i] = make([]uint8, width)
	}
	return newWorld
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
	c.ioCommand <- ioOutput
	outputFilename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(turn)
	c.ioFilename <- outputFilename
	readMutexSemaphore.Wait()
	//realReadMutex.Lock()

	//realReadMutex.Unlock()
	readMutexSemaphore.Post()
	//fmt.Println(world)

	if len(world) == 0 {
		return
	}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

}

func worker(startY, endY, startX, endX int, data func(y, x int) uint8, out workerChannels, p Params) {
	work, workFlipped := calculateSliceNextState(startY, endY, startX, endX, data, p)
	out.worldSlice <- work
	out.flippedCell <- workFlipped
}

func calculateSliceNextState(startY, endY, startX, endX int, data func(y, x int) uint8, p Params) ([][]uint8, []util.Cell) {
	height := endY - startY
	width := endX - startX

	nextSLice := MakeNewWorld(height, width)
	var flippedCell []util.Cell
	for i := startY; i < endY; i++ {
		for j := startX; j < endX; j++ {
			numberLive := 0
			for _, l := range [3]int{j - 1, j, j + 1} {
				for _, k := range [3]int{i - 1, i, i + 1} {
					newK := (k + p.ImageHeight) % p.ImageHeight
					newL := (l + p.ImageWidth) % p.ImageWidth
					if data(newK, newL) == 255 {
						numberLive++
					}
				}
			}
			if data(i, j) == 255 {
				numberLive -= 1
				if numberLive < 2 {
					nextSLice[i-startY][j-startX] = 0
					cell := util.Cell{X: j, Y: i}
					flippedCell = append(flippedCell, cell)
					//c.events <- CellFlipped{Cell: cell, CompletedTurns: turn}
				} else if numberLive > 3 {
					nextSLice[i-startY][j-startX] = 0
					cell := util.Cell{X: j, Y: i}
					flippedCell = append(flippedCell, cell)
					//c.events <- CellFlipped{Cell: cell, CompletedTurns: turn}
				} else {
					nextSLice[i-startY][j-startX] = 255
				}
			} else {
				if numberLive == 3 {
					nextSLice[i-startY][j-startX] = 255
					cell := util.Cell{X: j, Y: i}
					flippedCell = append(flippedCell, cell)
					//c.events <- CellFlipped{Cell: cell, CompletedTurns: turn}
				} else {
					nextSLice[i-startY][j-startX] = 0
				}
			}
		}
	}
	return nextSLice, flippedCell
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

func calculateNextState(world [][]uint8, p Params) ([][]uint8, []util.Cell) {
	data := MakeImmutableMatrix(world)
	// iterate through the cells
	var newPixelData [][]uint8
	var flipped []util.Cell
	if p.Threads == 1 {
		newPixelData, flipped = calculateSliceNextState(0, p.ImageHeight, 0, p.ImageWidth, data, p)
	} else {
		ChanSlice := make([]workerChannels, p.Threads)

		for i := 0; i < p.Threads; i++ {
			ChanSlice[i].worldSlice = make(chan [][]uint8)
			ChanSlice[i].flippedCell = make(chan []util.Cell)
		}
		for i := 0; i < p.Threads-1; i++ {
			go worker(int(float32(p.ImageHeight)*(float32(i)/float32(p.Threads))),
				int(float32(p.ImageHeight)*(float32(i+1)/float32(p.Threads))),
				0, p.ImageWidth, data, ChanSlice[i], p)
		}
		go worker(int(float32(p.ImageHeight)*(float32(p.Threads-1)/float32(p.Threads))),
			p.ImageHeight,
			0, p.ImageWidth, data, ChanSlice[p.Threads-1], p)

		MakeImmutableMatrix(newPixelData)
		for i := 0; i < p.Threads; i++ {

			part := <-ChanSlice[i].worldSlice
			newPixelData = append(newPixelData, part...)

			flippedPart := <-ChanSlice[i].flippedCell
			flipped = append(flipped, flippedPart...)
		}
	}
	return newPixelData, flipped
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

	turn := 0

	//fmt.Println(calculateAliveCells(p, world))
	for _, cell := range calculateAliveCells(p, world) {
		c.events <- CellFlipped{
			CompletedTurns: turn,
			Cell:           cell,
		}
	}
	//c.events <- TurnComplete{CompletedTurns: turn}

	// set the timer
	go timer(p, &world, &turn, c.events, &isEventChannelClosed)

	go checkKeyPresses(p, c, world, &turn, &isEventChannelClosed)

	// TODO: Execute all turns of the Game of Life.

	IPs := []string{"127.0.0.1:8030", "127.0.0.1:8040", "127.0.0.1:8050"}
	// iterate through the turns
	for t := 1; t <= p.Turns; t++ {
		chanSlice := make([]chan stubs.Response, len(IPs))

		for n := 0; n < len(IPs); n++ {
			chanSlice[n] = make(chan stubs.Response)
		}

		for n := 0; n < len(IPs)-1; n++ {
			go rpcWorker(IPs[n], p, world, n, len(IPs), chanSlice[n])
		}

		go rpcWorker(IPs[len(IPs)-1], p, world, len(IPs)-1, len(IPs), chanSlice[len(IPs)-1])

		var nextWorld [][]uint8
		var flipped []util.Cell

		for n := 0; n < len(IPs); n++ {
			response := <-chanSlice[n]
			nextWorld = append(nextWorld, response.NewWorld...)
			flipped = append(flipped, response.FlippedCell...)
		}
		//fmt.Println(flipped)

		//a parallel way to calculate all cells flipped
		for _, cell := range flipped {
			c.events <- CellFlipped{
				CompletedTurns: turn,
				Cell:           cell,
			}
		}
		//fmt.Println(turn)
		//time.Sleep(10 * time.Second)

		// this is a local way to calculate the cell flipped
		// inefficient but friendly and easy and local
		/*for h := 0; h < p.ImageHeight; h++ {
			for w := 0; w < p.ImageWidth; w++ {
				if world[h][w] != newPixelData[h][w] {
					cell := util.Cell{X: w, Y: h}
					c.events <- CellFlipped{
						CompletedTurns: turn,
						Cell:           cell,
					}
				}
			}
		}*/

		renderingSemaphore.Wait()
		c.events <- TurnComplete{CompletedTurns: turn}
		renderingSemaphore.Post()

		readMutexSemaphore.Wait()
		//realReadMutex.Lock()
		turn++
		world = nextWorld
		//realReadMutex.Unlock()
		readMutexSemaphore.Post()

	}

	// HANBIN: sometimes, is just not too good to to something too early
	//
	//
	//
	//fmt.Println("aaa")
	saveFile(c, p, world, turn)

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
