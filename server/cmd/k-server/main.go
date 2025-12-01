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

	"github.com/FraMan97/kairos/server/internal/api"
	"github.com/FraMan97/kairos/server/internal/config"
	"github.com/FraMan97/kairos/server/internal/crypto"
	"github.com/FraMan97/kairos/server/internal/database"
	"github.com/FraMan97/kairos/server/internal/service"
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

	_, err := service.StartTor()
	if err != nil {
		log.Println("[Main] - Starting error Tor: ", err)
		os.Exit(1)
	}

	err = crypto.GenerateKeyPair()
	if err != nil {
		log.Println("[Main] - Generation key pair error: ", err)
		os.Exit(1)
	}

	_, err = database.OpenDatabase()
	if err != nil {
		log.Println("[Main] - Error opening database: ", err)
		os.Exit(1)
	}

	err = database.EnsureBucket(config.BoltDB, "active_nodes")
	if err != nil {
		log.Println("[Main] - Error creating bucket 'active_nodes': ", err)
		os.Exit(1)
	}

	err = database.EnsureBucket(config.BoltDB, "manifests")
	if err != nil {
		log.Println("[Main] - Error creating bucket 'manifests': ", err)
		os.Exit(1)
	}

	go service.ServerBootstrapSync(ctx)

	go service.CleanOldRecords(ctx)

	http.HandleFunc("/subscribe", api.SubsribeNode)

	http.HandleFunc("/synchronize", api.SynchronizeData)

	http.HandleFunc("/file/nodes", api.RequestNodesForFileUpload)

	http.HandleFunc("/file/manifest", api.InsertFileManifest)

	http.HandleFunc("/manifests", api.DownloadFileManifest)

	log.Printf("[Main] - The bootstrap server is listening to localhost:%s\n", strconv.Itoa(config.Port))

	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil)
	if err != nil {
		log.Println("[Main] - Error Listening: ", err)
		os.Exit(1)
	}

}
