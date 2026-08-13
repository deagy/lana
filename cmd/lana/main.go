package main

import (
	"log"
	"os"

	"github.com/deagy/lana/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
