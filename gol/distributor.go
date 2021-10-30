package gol

import (
	"reflect"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
	"unsafe"
)

var isEventClosed bool

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// cited from stackoverflow
func isChanClosed(ch interface{}) bool {
	if reflect.TypeOf(ch).Kind() != reflect.Chan {
		panic("only channels!")
	}

	// get interface value pointer, from cgo_export
	// typedef struct { void *t; void *v; } GoInterface;
	// then get channel real pointer
	cptr := *(*uintptr)(unsafe.Pointer(
		unsafe.Pointer(uintptr(unsafe.Pointer(&ch)) + unsafe.Sizeof(uint(0))),
	))

	// this function will return true if chan.closed > 0
	// see hchan on https://github.com/golang/go/blob/master/src/runtime/chan.go
	// type hchan struct {
	// qcount   uint           // total data in the queue
	// dataqsiz uint           // size of the circular queue
	// buf      unsafe.Pointer // points to an array of dataqsiz elements
	// elemsize uint16
	// closed   uint32
	// **

	cptr += unsafe.Sizeof(uint(0)) * 2
	cptr += unsafe.Sizeof(unsafe.Pointer(uintptr(0)))
	cptr += unsafe.Sizeof(uint16(0))
	return *(*uint32)(unsafe.Pointer(cptr)) > 0
}

func timer(p Params, currentState *[][]uint8, turns *int, eventChan chan<- Event) {
	for {
		time.Sleep(2 * time.Second)
		number := len(calculateAliveCells(p, *currentState))

		if !isChanClosed(eventChan) {
			eventChan <- AliveCellsCount{CellsCount: number, CompletedTurns: *turns}
		} else {
			return
		}
	}
}

func makeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
}

// a function that create an empty world
func makeNewWorld(height, width int) [][]uint8 {
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

func worker(startY, endY, startX, endX int, data func(y, x int) uint8, out chan<- [][]uint8, p Params) {
	work := calculateNextState(startY, endY, startX, endX, data, p)
	out <- work
}

func calculateNextState(startY, endY, startX, endX int, data func(y, x int) uint8, p Params) [][]uint8 {
	height := endY - startY
	width := endX - startX

	nextSLice := makeNewWorld(height, width)

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
				} else if numberLive > 3 {
					nextSLice[i-startY][j-startX] = 0
				} else {
					nextSLice[i-startY][j-startX] = 255
				}
			} else {
				if numberLive == 3 {
					nextSLice[i-startY][j-startX] = 255
				} else {
					nextSLice[i-startY][j-startX] = 0
				}
			}
		}
	}
	return nextSLice
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	isEventClosed = false
	// let io start input
	c.ioCommand <- ioInput
	// HANBIN: send to the io goroutine the file name specified by the width and height
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
	c.ioFilename <- filename

	// TODO: Create a 2D slice to store the world.
	world := makeNewWorld(p.ImageHeight, p.ImageWidth)
	for h := 0; h < p.ImageHeight; h++ {
		for w := 0; w < p.ImageWidth; w++ {
			world[h][w] = <-c.ioInput
		}
	}

	turn := 0

	// set the timer
	go timer(p, &world, &turn, c.events)

	// TODO: Execute all turns of the Game of Life.
	// iterate through the turns
	if p.Turns > 0 {
		for t := 1; t <= p.Turns; t++ {
			data := makeImmutableMatrix(world)
			// iterate through the cells
			var newPixelData [][]uint8
			if p.Threads == 1 {
				newPixelData = calculateNextState(0, p.ImageHeight, 0, p.ImageWidth, data, p)

			} else {
				ChanSlice := make([]chan [][]uint8, p.Threads)
				for i := 0; i < p.Threads; i++ {
					ChanSlice[i] = make(chan [][]uint8)
				}
				for i := 0; i < p.Threads-1; i++ {
					go worker(int(float32(p.ImageHeight)*(float32(i)/float32(p.Threads))),
						int(float32(p.ImageHeight)*(float32(i+1)/float32(p.Threads))),
						0, p.ImageWidth, data, ChanSlice[i], p)
				}
				go worker(int(float32(p.ImageHeight)*(float32(p.Threads-1)/float32(p.Threads))),
					p.ImageHeight,
					0, p.ImageWidth, data, ChanSlice[p.Threads-1], p)
				makeImmutableMatrix(newPixelData)
				for i := 0; i < p.Threads; i++ {

					part := <-ChanSlice[i]

					newPixelData = append(newPixelData, part...)
				}
			}
			turn++
			world = newPixelData
		}
	}
	// HANBIN: sometimes, is just not too good to to something too early
	c.ioCommand <- ioOutput
	outputFilename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(turn)
	c.ioFilename <- outputFilename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// TODO: output proceeded map IO
	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	isEventClosed = true
	close(c.events)

}
