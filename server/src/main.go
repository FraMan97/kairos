package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/FraMan97/kairos/server/src/config"
	"github.com/FraMan97/kairos/server/src/service"
	"github.com/FraMan97/kairos/server/src/view"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	_, err := service.StartTor()
	if err != nil {
		log.Println("[Main] - Starting error Tor: ", err)
		os.Exit(1)
	}

	err = service.GenerateKeyPair()
	if err != nil {
		log.Println("[Main] - Generation key pair error: ", err)
		os.Exit(1)
	}

	_, err = service.OpenDatabase()
	if err != nil {
		log.Println("[Main] - Error opening database: ", err)
		os.Exit(1)
	}

	err = service.EnsureBucket(config.BoltDB, "active_nodes")
	if err != nil {
		log.Println("[Main] - Error creating bucket 'active_nodes': ", err)
		os.Exit(1)
	}

	err = service.EnsureBucket(config.BoltDB, "manifests")
	if err != nil {
		log.Println("[Main] - Error creating bucket 'manifests': ", err)
		os.Exit(1)
	}

	go service.ServerBootstrapSync(ctx, cancel)

	http.HandleFunc("/subscribe", view.Subscribe)

	http.HandleFunc("/synchronize", view.Synchronize)

	http.HandleFunc("/file/nodes", view.NodesForFileUpload)

	http.HandleFunc("/file/manifest", view.InsertManifest)

	http.HandleFunc("/manifests", view.DownloadManifest)

	log.Printf("[Main] - The bootstrap server is listening to localhost:%s\n", strconv.Itoa(config.Port))

	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil)
	if err != nil {
		log.Println("[Main] - Error Listening: ", err)
		os.Exit(1)
	}

}
