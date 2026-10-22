package main

import (
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/app"
	"log"
	"os"
)

func main() {
	if err := app.Run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
