package main

import (
	"fmt"
	"testing"
	"uk.ac.bris.cs/gameoflife/gol"
)

// Benchmark applies the filter to the ship.png b.N times.
// The time taken is carefully measured by go.
// The b.N  repetition is needed because benchmark results are not always constant.

func Benchmark(b *testing.B) {
	tests := []gol.Params{
		{ImageWidth: 512, ImageHeight: 512},
	}
	for _, p := range tests {
		for _, turns := range []int{1000} {
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
