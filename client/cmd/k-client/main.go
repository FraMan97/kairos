package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/FraMan97/kairos/client/internal/api"
	"github.com/FraMan97/kairos/client/internal/config"
	"github.com/FraMan97/kairos/client/internal/database"
	"github.com/FraMan97/kairos/client/internal/service"
)

func main() {
	bootstrapPtr := flag.String("bootstrap-servers", "", "bootstrap servers's .onion address (use the comma separator if many)")
	noBootstrapPtr := flag.Bool("no-bootstrap-servers", false, "Start the bootstrap server without other bootstrap servers (standalone mode)")
	flag.Parse()

	if err := config.InitConfig(); err != nil {
		log.Println("[Config] - Error initializing config: ", err)
		os.Exit(1)
	}

	if *bootstrapPtr != "" {
		config.BootStrapServers = strings.Split(*bootstrapPtr, ",")
		log.Println("[Config] - Bootstrap Servers set using flag: ", config.BootStrapServers)
	} else if *noBootstrapPtr {
		config.BootStrapServers = []string{}
		log.Println("[Config] - Mode 'standalone' activated")
	} else if len(config.BootStrapServers) == 0 {
		log.Println("[Config] - No Boostrap Servers set. Set using --bootstrap-servers flag")
		os.Exit(1)
	}

	ctx, _ := context.WithCancel(context.Background())
	_, err := database.OpenDatabase()
	if err != nil {
		log.Println("[Main] - Error opening database: ", err)
		os.Exit(1)
	}

	err = database.EnsureBucket(config.BoltDB, "chunks")
	if err != nil {
		log.Println("[Main] - Error creating bucket 'chunks': ", err)
		os.Exit(1)
	}

	http.HandleFunc("/start", api.StartNode)

	http.HandleFunc("/put", api.PutFile)

	http.HandleFunc("/get", api.GetFile)

	http.HandleFunc("/chunk", api.Chunk)

	go service.CleanOldRecords(ctx)

	log.Printf("[Main] - The Kairos node is listening to localhost:%s\n", strconv.Itoa(config.Port))

	err = http.ListenAndServe(fmt.Sprintf(":%s", strconv.Itoa(config.Port)), nil)
	if err != nil {
		log.Println("[Main] - Error Listening: ", err)
		os.Exit(1)
	}
}
