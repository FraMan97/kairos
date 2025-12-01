package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"slices"
	"time"

	"github.com/FraMan97/kairos/server/internal/config"
	"github.com/FraMan97/kairos/server/internal/database"
	"github.com/FraMan97/kairos/server/internal/models"
)

func CleanOldRecords(ctx context.Context) {
	ticker := time.NewTicker(getDelay(config.CronClean))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			clean()

		case <-ctx.Done():
			log.Println("[Sync] - Context cancelled, stopping ticker")
			return
		}
	}
}

func clean() {
	allManifestsData, err := database.GetAllData(config.BoltDB, "manifests")
	if err != nil {
		log.Println("[Clean] - Error: ", err)
		return
	}
	var manifest models.FileManifest
	var parsedTime time.Time
	var now time.Time
	for _, m := range allManifestsData {
		json.NewDecoder(bytes.NewBuffer(m)).Decode(&manifest)
		parsedTime, err = time.Parse(time.RFC3339, manifest.ReleaseDate)
		if err != nil {
			log.Println("[Clean] - Error: ", err)
			continue
		}
		oneWeekLater := parsedTime.Add(time.Hour * 24 * 7) // clean old manifests after 1 week
		now = time.Now().UTC()
		if now.After(oneWeekLater) {
			err = database.DeleteKey(config.BoltDB, "manifests", manifest.FileId)
			if err != nil {
				log.Println("[Clean] - Error: ", err)
			}
		}
	}

	allManifestsData, err = database.GetAllData(config.BoltDB, "manifests")
	if err != nil {
		log.Println("[Clean] - Error: ", err)
		return
	}

	allActiveNodes, err := database.GetAllKeys(config.BoltDB, "active_nodes")
	if err != nil {
		log.Println("[Clean] - Error: ", err)
		return
	}
	for _, a := range allActiveNodes {
		found := false
		for _, m := range allManifestsData {
			json.NewDecoder(bytes.NewBuffer(m)).Decode(&manifest)
			for _, b := range manifest.Split {
				for _, c := range b.Chunks {
					if slices.Contains(c.Nodes, a) {
						found = true
					}
				}
			}
		}
		if found {
			err = database.DeleteKey(config.BoltDB, "active_nodes", a)
			if err != nil {
				log.Println("[Clean] - Error: ", err)
			}
		}
	}
}
