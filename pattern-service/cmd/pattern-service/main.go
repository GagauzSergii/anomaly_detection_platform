package main

import (
	"log"
	"os"
)

func main() {
	if err := run(); err != nil {
		log.Printf("metrics service shutdown with error: %v\n", err)
		os.Exit(1)
	}
}
