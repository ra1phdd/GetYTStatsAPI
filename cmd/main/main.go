package main

import (
	"getytstatsapi/internal/pkg/app"
	"log"
)

func main() {
	err := app.New()
	if err != nil {
		log.Fatal(err)
	}
}
