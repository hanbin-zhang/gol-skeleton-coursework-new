package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolOperations struct{}

type workerChannels struct {
	worldSlice  chan [][]uint8
	flippedCell chan []util.Cell
}

func MakeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
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

func (g *GolOperations) ShutDownServer(req stubs.Request, res *stubs.Response) (err error) {
	fmt.Println("server exiting...")
	os.Exit(2)
	return
}

func calculateNextState(req stubs.Request) []util.Cell {
	threads := req.Threads
	world := req.World
	data := MakeImmutableMatrix(world)
	// iterate through the cells
	var newPixelData [][]uint8
	var flipped []util.Cell

	if threads == 1 {
		newPixelData, flipped = calculateSliceNextState(req.StartY, req.EndY, req.StartX, req.EndX, req.ImageHeight, req.ImageWidth, data)
	} else {
		ChanSlice := make([]workerChannels, threads)
		ImageHeight := req.EndY - req.StartY

		for i := 0; i < threads; i++ {
			ChanSlice[i].worldSlice = make(chan [][]uint8)
			ChanSlice[i].flippedCell = make(chan []util.Cell)
		}
		for i := 0; i < threads-1; i++ {
			go worker(int(float32(ImageHeight)*(float32(i)/float32(threads)))+req.StartY,
				int(float32(ImageHeight)*(float32(i+1)/float32(threads)))+req.StartY,
				req.StartX, req.EndX, req.ImageHeight, req.ImageWidth, data, ChanSlice[i])
		}
		go worker(int(float32(ImageHeight)*(float32(threads-1)/float32(threads)))+req.StartY,
			ImageHeight+req.StartY,
			req.StartX, req.EndX, req.ImageHeight, req.ImageWidth, data, ChanSlice[threads-1])

		//MakeImmutableMatrix(newPixelData)
		for i := 0; i < threads; i++ {
			part := <-ChanSlice[i].worldSlice
			newPixelData = append(newPixelData, part...)

			flippedPart := <-ChanSlice[i].flippedCell
			flipped = append(flipped, flippedPart...)
		}
	}
	return flipped
}

func (g *GolOperations) CalculateCellFlipped(req stubs.Request, res *stubs.Response) (err error) {

	// iterate through the cells
	flippedCell := calculateNextState(req)

	res.FlippedCell = flippedCell
	res.SliceNumber = req.SliceNumber
	return
}

func main() {
	pAddr := flag.String("port", "127.0.0.1:8040", "Port to listen on")
	bAddr := flag.String("broker", "127.0.0.1:8030", "IP of broker")
	flag.Parse()

	err := rpc.Register(&GolOperations{})
	if err != nil {
		fmt.Println(err)
		return
	}

	client, _ := rpc.Dial("tcp", *bAddr)
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	}(client)

	subscribe := stubs.Subscription{NodeAddress: *pAddr, Callback: stubs.GolHandler}
	res := new(stubs.StatusReport)
	_ = client.Go(stubs.Subscribe, subscribe, res, nil)

	listener, _ := net.Listen("tcp", *pAddr)
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
