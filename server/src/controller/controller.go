package controller

import (
	"crypto/sha256"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/FraMan97/kairos/server/src/config"
	"github.com/FraMan97/kairos/server/src/model"
	"github.com/FraMan97/kairos/server/src/service"
)

func SubsribeNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Println("[Subscribe] - Only POST method allowed!")
		http.Error(w, "Only POST Method allowed!", http.StatusMethodNotAllowed)
		return
	}

	var subscription model.SubscriptionRequest

	err := json.NewDecoder(r.Body).Decode(&subscription)
	if err != nil {
		log.Println("[Subscribe] - Invalid JSON:", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("[Subscribe] - Received request from %s \n", subscription.Address)

	defer r.Body.Close()

	message, err := json.Marshal(model.SubscriptionRequest{Address: subscription.Address, PublicKey: subscription.PublicKey})
	if err != nil {
		log.Println("[Subscribe] - Invalid serialization:", err)
		http.Error(w, "Invalid serialization", http.StatusBadRequest)
		return
	}

	check, err := service.VerifySignature(message, subscription.Signature, subscription.PublicKey)
	if err != nil {
		log.Println("[Subscribe] - Invalid verification signature:", err)
		http.Error(w, "Invalid verification signature", http.StatusBadRequest)
		return
	}
	if check {
		payload, err := json.Marshal(model.ActiveNodeRecord{PublicKey: subscription.PublicKey, Timestamp: time.Now().UnixNano()})
		if err != nil {
			log.Println("[Subscribe] - Error preparing payload:", err)
			http.Error(w, "Error preparing payload", http.StatusInternalServerError)
			return
		}
		err = service.PutData(config.BoltDB, "active_nodes", subscription.Address, payload)
		if err != nil {
			log.Println("[Subscribe] - Error inserting data:", err)
			http.Error(w, "Error inserting data", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Sender not verified", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func SynchronizeData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Println("[Sync] - Only POST method allowed!")
		http.Error(w, "Only POST Method allowed!", http.StatusMethodNotAllowed)
		return
	}

	var receivedData model.SynchronizationRequest

	err := json.NewDecoder(r.Body).Decode(&receivedData)
	if err != nil {
		log.Println("[Sync] - Invalid JSON:", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("[Sync] - Received request from %s\n", receivedData.Address)

	defer r.Body.Close()

	message, err := json.Marshal(model.SynchronizationRequest{Address: receivedData.Address, PublicKey: receivedData.PublicKey,
		ActiveNodes: receivedData.ActiveNodes, FileManifests: receivedData.FileManifests})
	if err != nil {
		log.Println("[Sync] - Invalid serialization:", err)
		http.Error(w, "Invalid serialization", http.StatusBadRequest)
		return
	}

	check, err := service.VerifySignature(message, receivedData.Signature, receivedData.PublicKey)
	if err != nil {
		log.Println("[Sync] - Invalid verification signature:", err)
		http.Error(w, "Invalid verification signature", http.StatusBadRequest)
		return
	}

	if check {
		activeNodes, err := service.GetAllData(config.BoltDB, "active_nodes")
		if err != nil {
			log.Println("[Sync] - Error get all data from bucket 'active_nodes':", err)
			http.Error(w, "Error get all data from bucket 'active_nodes'", http.StatusInternalServerError)
			return
		}

		fileManifests, err := service.GetAllData(config.BoltDB, "manifests")
		if err != nil {
			log.Println("[Sync] - Error get all data from bucket 'manifests':", err)
			http.Error(w, "Error get all data manifests", http.StatusInternalServerError)
			return
		}

		dataToExchange := model.SynchronizationRequest{Address: config.OnionAddress + strconv.Itoa(config.Port), PublicKey: config.PublicKey,
			ActiveNodes: activeNodes, FileManifests: fileManifests}

		jsonBytes, err := json.Marshal(dataToExchange)
		if err != nil {
			log.Println("[Sync] - Invalid serialization:", err)
			http.Error(w, "Invalid serialization", http.StatusBadRequest)
			return
		}

		signature, err := service.SignMessage(jsonBytes)
		if err != nil {
			log.Println("[Sync] - Error signing message:", err)
			http.Error(w, "Error signing message", http.StatusBadRequest)
			return
		}
		dataToExchange.Signature = signature

		jsonBytes, err = json.Marshal(dataToExchange)
		if err != nil {
			log.Println("[Sync] - Invalid serialization:", err)
			http.Error(w, "Invalid serialization", http.StatusBadRequest)
			return
		}

		go service.ProcessAlignment(dataToExchange, receivedData)

		w.Write(jsonBytes)
	} else {
		http.Error(w, "Sender not verified", http.StatusUnauthorized)
		return
	}

}

func RequestNodesForFileUpload(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if r.Method != http.MethodPost {
		log.Println("[ReqNodes] - Only POST method allowed!")
		http.Error(w, "Only POST Method allowed!", http.StatusMethodNotAllowed)
		return
	}

	var request model.NodesForFileUploadRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		log.Println("[ReqNodes] - Invalid JSON:", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("[ReqNodes] - Received request from %s\n", request.Address)

	message, err := json.Marshal(model.NodesForFileUploadRequest{Address: request.Address, PublicKey: request.PublicKey,
		TotalChunks: request.TotalChunks, NodesPerChunk: request.NodesPerChunk})
	if err != nil {
		log.Println("[ReqNodes] - Invalid serialization:", err)
		http.Error(w, "Invalid serialization", http.StatusBadRequest)
		return
	}

	check, err := service.VerifySignature(message, request.Signature, request.PublicKey)
	if err != nil {
		log.Println("[ReqNodes] - Invalid verification signature:", err)
		http.Error(w, "Invalid verification signature", http.StatusBadRequest)
		return
	}

	if check {
		allDBNodes, err := service.GetAllKeys(config.BoltDB, "active_nodes")
		if err != nil {
			log.Println("[ReqNodes] - Error get all data from bucket 'active_nodes':", err)
			http.Error(w, "Error get all data from bucket 'active_nodes'", http.StatusInternalServerError)
			return
		}
		var response []string = []string{}

		nodesToPickup := int(math.Ceil(float64(request.TotalChunks) / float64(request.NodesPerChunk)))
		if len(allDBNodes) <= nodesToPickup {
			response = append(response, allDBNodes...)
		} else {
			if nodesToPickup > config.MaxNodesReturned {
				response = pickRandomItems(allDBNodes, config.MaxNodesReturned)
			} else {
				response = pickRandomItems(allDBNodes, nodesToPickup)
			}
		}
		jsonBytes, err := json.Marshal(response)
		if err != nil {
			log.Println("[ReqNodes] - Invalid serialization:", err)
			http.Error(w, "Invalid serialization", http.StatusBadRequest)
			return
		}
		w.Write(jsonBytes)
	} else {
		http.Error(w, "Sender not verified", http.StatusUnauthorized)
		return
	}

}

func InsertFileManifest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if r.Method != http.MethodPost {
		log.Println("[InsFileManifest] - Only POST method allowed!")
		http.Error(w, "Only POST Method allowed!", http.StatusMethodNotAllowed)
		return
	}

	var request model.FileManifestRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		log.Println("[InsFileManifest] - Invalid JSON:", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("[InsFileManifest] - Received request from %s a\n", request.Address)

	var fileManifest model.FileManifest = request.Manifest
	manifestBytes, err := json.Marshal(fileManifest)
	if err != nil {
		log.Println("[InsFileManifest] - Invalid serialization for verify:", err)
		http.Error(w, "Invalid serialization for verify", http.StatusBadRequest)
		return
	}

	hashToVerify := sha256.Sum256(manifestBytes)

	check, err := service.VerifySignature(hashToVerify[:], request.Signature, request.PublicKey)
	if err != nil {
		log.Println("[InsFileManifest] - Invalid verification signature:", err)
		http.Error(w, "Invalid verification signature", http.StatusBadRequest)
		return
	}

	if check {
		err = service.PutData(config.BoltDB, "manifests", fileManifest.FileId, manifestBytes)
		if err != nil {
			log.Println("[InsFileManifest] - Error inserting data in bucket 'manifests':", err)
			http.Error(w, "Error inserting data in bucket 'manifests'", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	} else {
		http.Error(w, "Sender not verified", http.StatusUnauthorized)
		return
	}
}

func DownloadFileManifest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if r.Method != http.MethodPost {
		log.Println("[DowFileManifest] - Only POST method allowed!")
		http.Error(w, "Only POST Method allowed!", http.StatusMethodNotAllowed)
		return
	}

	var request model.GetFileManifestRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		log.Println("[DowFileManifest] - Invalid JSON:", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("[DowFileManifest] - Received request from %s \n", request.Address)

	message, err := json.Marshal(model.GetFileManifestRequest{Address: request.Address, PublicKey: request.PublicKey,
		FileId: request.FileId})
	if err != nil {
		log.Println("[DowFileManifest] - Invalid serialization:", err)
		http.Error(w, "Invalid serialization", http.StatusBadRequest)
		return
	}

	check, err := service.VerifySignature(message, request.Signature, request.PublicKey)
	if err != nil {
		log.Println("[DowFileManifest] - Invalid verification signature:", err)
		http.Error(w, "Invalid verification signature", http.StatusBadRequest)
		return
	}

	if check {
		dbData, err := service.GetData(config.BoltDB, "manifests", request.FileId)
		if err != nil {
			log.Println("[DowFileManifest] - Error get manifest from DB: ", err)
			http.Error(w, "Error get manifestfrom DB", http.StatusInternalServerError)
			return
		}
		w.Write(dbData)
	} else {
		http.Error(w, "Sender not verified", http.StatusUnauthorized)
		return
	}

}

func pickRandomItems(list []string, n int) []string {
	selected := make([]string, 0, n)

	if n > len(list) {
		n = len(list)
	}

	for i := 0; i < n; i++ {
		randomIndex := rand.Intn(len(list))
		selected = append(selected, list[randomIndex])

		list[randomIndex] = list[len(list)-1]
		list = list[:len(list)-1]
	}

	return selected
}
