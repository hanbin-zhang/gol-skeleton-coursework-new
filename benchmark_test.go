package main

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"testing"
	"uk.ac.bris.cs/gameoflife/gol"
	"os"
	"uk.ac.bris.cs/gameoflife/util"
)

func readPgmImage(name string, Width int, Height int, input chan uint8) {
	// Request a filename from the distributor.
	filename := name

	data, ioError := ioutil.ReadFile("images/" + filename + ".pgm")
	util.Check(ioError)

	fields := strings.Fields(string(data))

	if fields[0] != "P5" {
		panic("Not a pgm file")
	}

	width, _ := strconv.Atoi(fields[1])
	if width != Width {
		panic("Incorrect width")
	}

	height, _ := strconv.Atoi(fields[2])
	if height != Height {
		panic("Incorrect height")
	}

	maxval, _ := strconv.Atoi(fields[3])
	if maxval != 255 {
		panic("Incorrect maxval/bit depth")
	}

	image := []byte(fields[4])

	for _, b := range image {
		input <- b
	}

	fmt.Println("File", filename, "input done!")
}

// The time taken is carefully measured by go.
// The b.N  repetition is needed because benchmark results are not always constant.
/*func BenchmarkCalculateNextState(b *testing.B) {
	os.Stdout = nil
	height := 512
	width := 512
	input := make(chan uint8)
	go readPgmImage("512x512", width, height, input)
	world := gol.MakeNewWorld(height, width)
	for h := 0; h < height; h++ {
		for w := 0; w < width; w++ {
			world[h][w] = <-input
		}
	}
	for n := 1; n <= 16; n++ {
		b.Run(fmt.Sprintf("%d_threads", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ow := world
				for j := 1;j<=1000;j++ {

					nw,_ := gol.CalculateNextState(ow, gol.Params{Turns: 1000, ImageWidth: 512, Threads: n, ImageHeight: 512})
					ow = nw
				}

			}
		})
	}
}*/

func Benchmark(b *testing.B) {
    os.Stdout = nil
	tests := []gol.Params{
		{ImageWidth: 512, ImageHeight: 512},
	}
	for _, p := range tests {
		for _, turns := range []int{100} {
			p.Turns = turns

			for threads := 1; threads <= 16; threads++ {
				p.Threads = threads
				testName := fmt.Sprintf("%dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
				b.Run(testName, func(b *testing.B) {
					events := make(chan gol.Event)
					go gol.Run(p, events, nil)
					for event := range events {
						switch e := event.(type) {
						case gol.FinalTurnComplete:
							fmt.Println(e)
							break
						}
					}
				})
			}
		}
		// os.Exit(2)
	}
}
