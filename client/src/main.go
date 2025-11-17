package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/FraMan97/kairos/client/src/config"
	"github.com/FraMan97/kairos/client/src/service"
	"github.com/FraMan97/kairos/client/src/view"
)

func main() {
	_, err := service.OpenDatabase()
	if err != nil {
		log.Println("[Main] - Error opening database: ", err)
		os.Exit(1)
	}

	err = service.EnsureBucket(config.BoltDB, "chunks")
	if err != nil {
		log.Println("[Main] - Error creating bucket 'chunks': ", err)
		os.Exit(1)
	}

	http.HandleFunc("/start", view.Start)

	http.HandleFunc("/put", view.Put)

	http.HandleFunc("/get", view.Get)

	http.HandleFunc("/chunk", view.Chunk)

	log.Printf("[Main] - The Kairos node is listening to localhost:%s\n", strconv.Itoa(config.Port))

	err = http.ListenAndServe(fmt.Sprintf(":%s", strconv.Itoa(config.Port)), nil)
	if err != nil {
		log.Println("[Main] - Error Listening: ", err)
		os.Exit(1)
	}
}
