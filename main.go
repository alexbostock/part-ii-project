package main

import (
	"log"
	"os"
	"strconv"

	"github.com/alexbostock/part-ii-project/simnet"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: part-ii-project <number of db nodes>")
	}

	n, err := strconv.ParseUint(os.Args[1], 10, 32)

	if err != nil {
		log.Fatal("Usage: part-ii-project <number of db nodes>")
	}

	if n < 1 {
		log.Fatal("Number of nodes must be a positive integer")
	}

	simnet.Simulate(uint(n))
}
