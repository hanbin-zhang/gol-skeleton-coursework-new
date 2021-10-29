package gol

import (
	"fmt"
	"strconv"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
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

func calculateNextState(startY, endY, startX, endX int, data func(y, x int) uint8, p Params) [][]uint8 {
	height := endY - startY
	width := endX - startX

	fmt.Println(height)
	fmt.Println(width)
	nextSLice := makeNewWorld(height, width)
	numberLive := 0

	for i := startY; i < endY; i++ {
		for j := startX; j < endX; j++ {
			for k := i - 1; k <= i+1; k++ {
				for l := j - 1; l <= j+1; l++ {
					newK := (k + p.ImageHeight) % p.ImageHeight
					newL := (l + p.ImageWidth) % p.ImageWidth

					if data(newK, newL) == 255 {
						numberLive += 1

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
	// let io start input
	c.ioCommand <- ioInput
	// HANBIN: send to the io goroutine the file name specified by the width and height
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
	c.ioFilename <- filename

	// TODO: Create a 2D slice to store the world.
	initWorld := makeNewWorld(p.ImageHeight, p.ImageWidth)
	for h := 0; h < p.ImageHeight; h++ {
		for w := 0; w < p.ImageWidth; w++ {
			initWorld[h][w] = <-c.ioInput
		}
	}

	turn := 0
	world := initWorld

	// TODO: Execute all turns of the Game of Life.
	// iterate through the turns
	for t := 1; t <= p.Turns; t++ {
		newWorld := makeNewWorld(p.ImageHeight, p.ImageWidth)
		data := makeImmutableMatrix(world)
		// iterate through the cells
		if p.Threads == 1 {
			world = calculateNextState(0, p.ImageHeight, 0, p.ImageWidth, data, p)
		} else {
			world = newWorld
		}
	}

	c.ioCommand <- ioOutput
	outputFilename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "_output"
	c.ioFilename <- outputFilename
	for _, column := range world {
		for cell, _ := range column {
			c.ioOutput <- uint8(cell)
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
	close(c.events)
}
