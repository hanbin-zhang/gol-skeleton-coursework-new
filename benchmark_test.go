package main

import (
	"os"
	"testing"
)

// The time taken is carefully measured by go.
// The b.N  repetition is needed because benchmark results are not always constant.
func BenchmarkCalculateNextState(b *testing.B) {
	os.Stdout = nil
	for n := 0; n < b.N; n++ {

	}
}

func BenchmarkPgm(b *testing.B) {
	os.Stdout = nil
	for n := 0; n < b.N; n++ {
	}
}

func BenchmarkGol(b *testing.B) {
	os.Stdout = nil
	for n := 0; n < b.N; n++ {
	}
}
