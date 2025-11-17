package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/FraMan97/kairos/server/src/config"
	"github.com/FraMan97/kairos/server/src/model"
)

func ServerBootstrapSync(ctx context.Context, cancel context.CancelFunc) {
	ticker := time.NewTicker(getDelay())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sync()

		case <-ctx.Done():
			log.Println("[Sync] - Context cancelled, stopping ticker")
			cancel()
			return
		}
	}
}

func sync() {
	chosenServer := 0
	for {
		chosenServer = rand.Intn(len(config.BootStrapServers))
		if config.BootStrapServers[chosenServer] != config.OnionAddress+":"+strconv.Itoa(config.Port) {
			break
		}
	}
	activeNodes, err := GetAllData(config.BoltDB, "active_nodes")
	if err != nil {
		log.Println("[Sync] - Error: ", err)
		return
	}

	fileManifests, err := GetAllData(config.BoltDB, "manifests")
	if err != nil {
		log.Println("[Sync] - Error: ", err)
		return
	}

	dataToExchange := model.SynchronizationRequest{Address: config.OnionAddress + strconv.Itoa(config.Port), PublicKey: config.PublicKey,
		ActiveNodes: activeNodes, FileManifests: fileManifests}

	jsonBytes, err := json.Marshal(dataToExchange)
	if err != nil {
		log.Println("[Sync] - Error: ", err)
		return
	}

	signature, err := SignMessage(jsonBytes)
	if err != nil {
		log.Println("[Sync] - Error: ", err)
		return
	}
	dataToExchange.Signature = signature

	jsonBytes, err = json.Marshal(dataToExchange)
	if err != nil {
		log.Println("[Sync] - Error: ", err)
		return
	}
	var receivedData model.SynchronizationRequest
	resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/synchronize", config.BootStrapServers[chosenServer]),
		"application/json",
		bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Println("[sync] - Error: ", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		json.NewDecoder(resp.Body).Decode(&receivedData)
		ProcessAlignment(dataToExchange, receivedData)
		return
	} else {
		log.Println("[Sync] - Error:", resp.Body)
	}
}

func getDelay() time.Duration {
	jitter := time.Duration(rand.Intn(config.CronSync)) * time.Second
	delay := jitter + time.Duration(config.CronSync)*time.Second
	return delay
}

func ProcessAlignment(dbData model.SynchronizationRequest, receivedData model.SynchronizationRequest) {
	for k := range receivedData.ActiveNodes {
		check, _ := ExistsKey(config.BoltDB, "active_nodes", k)
		if check {
			var current *model.ActiveNodeRecord
			var received *model.ActiveNodeRecord
			json.Unmarshal(dbData.ActiveNodes[k], &current)
			json.Unmarshal(receivedData.ActiveNodes[k], &received)

			if current.Timestamp < received.Timestamp {
				PutData(config.BoltDB, "active_nodes", k, receivedData.ActiveNodes[k])
			}

		} else {
			PutData(config.BoltDB, "active_nodes", k, receivedData.ActiveNodes[k])
		}
	}

	for k := range receivedData.FileManifests {
		check, _ := ExistsKey(config.BoltDB, "manifests", k)
		if !check {
			var received *model.FileManifest
			json.Unmarshal(receivedData.FileManifests[k], &received)
			PutData(config.BoltDB, "manifests", k, receivedData.FileManifests[k])
		}

	}

}
