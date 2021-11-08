package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct{}

type workerChannels struct {
	worldSlice  chan [][]uint8
	flippedCell chan []util.Cell
}

func worker(startY, endY, startX, endX, ImageHeight, ImageWidth int, data func(y, x int) uint8, out workerChannels) {
	work, workFlipped := calculateSliceNextState(startY, endY, startX, endX, ImageHeight, ImageWidth, data)
	out.worldSlice <- work
	out.flippedCell <- workFlipped
}

func calculateSliceNextState(startY, endY, startX, endX, ImageHeight, ImageWidth int, data func(y, x int) uint8) ([][]uint8, []util.Cell) {
	height := endY - startY
	width := endX - startX

	nextSLice := gol.MakeNewWorld(height, width)
	var flippedCell []util.Cell
	for i := startY; i < endY; i++ {
		for j := startX; j < endX; j++ {
			numberLive := 0
			for _, l := range [3]int{j - 1, j, j + 1} {
				for _, k := range [3]int{i - 1, i, i + 1} {
					newK := (k + ImageHeight) % ImageHeight
					newL := (l + ImageWidth) % ImageWidth
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

func calculateNextState(req stubs.Request, world [][]uint8) ([][]uint8, []util.Cell) {
	data := gol.MakeImmutableMatrix(world)
	// iterate through the cells
	var newPixelData [][]uint8
	var flipped []util.Cell
	if p.Threads == 1 {
		newPixelData, flipped = calculateSliceNextState(0, ImageHeight, 0, p.ImageWidth, data, p)
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

func (g *GolOperations) CalculateCellFlipped(req stubs.Request, res *stubs.Response) (err error) {

	world := req.World

	// iterate through the cells

	height := req.EndY - req.StartY
	width := req.EndX - req.StartX

	data := gol.MakeImmutableMatrix(world)
	nextSLice := gol.MakeNewWorld(height, width)
	var flippedCell []util.Cell
	for i := req.StartY; i < req.EndY; i++ {
		for j := req.StartX; j < req.EndX; j++ {
			numberLive := 0
			for _, l := range [3]int{j - 1, j, j + 1} {
				for _, k := range [3]int{i - 1, i, i + 1} {
					newK := (k + req.ImageHeight) % req.ImageHeight
					newL := (l + req.ImageWidth) % req.ImageWidth
					if data(newK, newL) == 255 {
						numberLive++
					}
				}
			}
			if data(i, j) == 255 {
				numberLive -= 1
				if numberLive < 2 {
					nextSLice[i-req.StartY][j-req.StartX] = 0
					cell := util.Cell{X: j, Y: i}
					flippedCell = append(flippedCell, cell)
					//c.events <- CellFlipped{Cell: cell, CompletedTurns: turn}
				} else if numberLive > 3 {
					nextSLice[i-req.StartY][j-req.StartX] = 0
					cell := util.Cell{X: j, Y: i}
					flippedCell = append(flippedCell, cell)
					//c.events <- CellFlipped{Cell: cell, CompletedTurns: turn}
				} else {
					nextSLice[i-req.StartY][j-req.StartX] = 255
				}
			} else {
				if numberLive == 3 {
					nextSLice[i-req.StartY][j-req.StartX] = 255
					cell := util.Cell{X: j, Y: i}
					flippedCell = append(flippedCell, cell)
					//c.events <- CellFlipped{Cell: cell, CompletedTurns: turn}
				} else {
					nextSLice[i-req.StartY][j-req.StartX] = 0
				}
			}
		}
	}
	res.NewWorld = nextSLice
	res.FlippedCell = flippedCell
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	err := rpc.Register(&GolOperations{})
	if err != nil {
		fmt.Println(err)
		return
	}
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
	}(listener)

	fmt.Println("Server ready:", *pAddr)
	rpc.Accept(listener)

}
