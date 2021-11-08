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
