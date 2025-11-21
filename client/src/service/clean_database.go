package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/FraMan97/kairos/client/src/config"
	"github.com/FraMan97/kairos/client/src/model"
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
	allChunksData, err := GetAllData(config.BoltDB, "chunks")
	if err != nil {
		log.Println("[Clean] - Error: ", err)
		return
	}
	var chunk model.ChunkRequest
	var parsedTime time.Time
	var now time.Time
	for _, m := range allChunksData {
		json.NewDecoder(bytes.NewBuffer(m)).Decode(&chunk)
		parsedTime, err = time.Parse(time.RFC3339, chunk.ReleaseDate)
		if err != nil {
			log.Println("[Clean] - Error: ", err)
			continue
		}
		oneWeekLater := parsedTime.Add(time.Hour * 24 * 7) // clean old chunks after 1 week
		now = time.Now().UTC()
		if now.After(oneWeekLater) {
			err = DeleteKey(config.BoltDB, "chunks", chunk.ChunkId)
			if err != nil {
				log.Println("[Clean] - Error: ", err)
			}
		}
	}
}

func getDelay(cron int) time.Duration {
	jitter := time.Duration(rand.Intn(cron)) * time.Second
	delay := jitter + time.Duration(cron)*time.Second
	return delay
}
