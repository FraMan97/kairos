package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/FraMan97/kairos/client/internal/config"
	"github.com/FraMan97/kairos/client/internal/crypto"
	"github.com/FraMan97/kairos/client/internal/models"
	"github.com/FraMan97/kairos/client/internal/service"
)

func StartNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Println("[StartNode] - Only POST method allowed!")
		http.Error(w, "Only POST method allowed!", http.StatusMethodNotAllowed)
		return
	}
	log.Println("[StartNode] - Starting Kairos Node...")
	_, err := service.StartTor()
	if err != nil {
		log.Println("[StartNode] - Starting error TOR: ", err)
		http.Error(w, "Starting error TOR", http.StatusInternalServerError)
		return
	}

	err = crypto.GenerateKeyPair()
	if err != nil {
		log.Println("[StartNode] - Generation key pair error: ", err)
		http.Error(w, "Error generation key pair", http.StatusInternalServerError)
		return
	}

	err = service.SubscribeNode()
	if err != nil {
		log.Println("[StartNode] - Error subscription Kairos node to BootstrapServer: ", err)
		http.Error(w, "Error subcription Kairos node to BootstrapServer", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func PutFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Println("[PutFile] - Only POST method allowed!")
		http.Error(w, "Only POST method allowed!", http.StatusMethodNotAllowed)
		return
	}

	log.Println("[PutFile] - Putting file in the Kairos network...")

	blockSize := config.TargetChunkSize * config.DataShards

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Println("[PutFile] - Creation form file error: ", err)
		http.Error(w, "Creation form file error", http.StatusInternalServerError)
		return
	}

	releaseTime := r.FormValue("release_time")

	defer file.Close()
	results, blockSizes, err := service.SplitFile(file, blockSize, releaseTime)
	if err != nil {
		log.Println("[PutFile] - Splitting file error: ", err)
		http.Error(w, "Splitting file error", http.StatusInternalServerError)
		return
	}

	nodes, err := service.RequestNodesForFileUpload(len(results) * config.TotalShards)
	if err != nil {
		log.Println("[PutFile] - Requiring nodes error: ", err)
		http.Error(w, "Requiring nodes error", http.StatusInternalServerError)
		return
	}

	fileManifest, err := service.GenerateFileManifest(results, blockSizes, nodes, file, header, releaseTime)
	if err != nil {
		log.Println("[PutFile] - Generating file manifest error: ", err)
		http.Error(w, "Generating file manifest error", http.StatusInternalServerError)
		return
	}

	err = service.UploadFileManifest(fileManifest)
	if err != nil {
		log.Println("[PutFile] - Uploading file manifest error: ", err)
		http.Error(w, "Uploading file manifest error", http.StatusInternalServerError)
		return
	}

	err = service.UploadFile(fileManifest, results)
	if err != nil {
		log.Println("[PutFile] - Uploading file error: ", err)
		http.Error(w, "Uploading file error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, fileManifest.FileId)
}

func GetFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Println("[GetFile] - Only GET method allowed!")
		http.Error(w, "Only POST method allowed!", http.StatusMethodNotAllowed)
		return
	}

	fileId := r.URL.Query().Get("fileId")
	if fileId == "" {
		log.Println("[GetFile] - Missing fileId query parameter")
		http.Error(w, "Missing fileId query parameter", http.StatusBadRequest)
		return
	}

	fileManifest, err := service.GetFileManifestFromServer(fileId)
	if err != nil {
		log.Printf("Error retrieving file manifest from the Bootstrap Server: %v\n", err)
		http.Error(w, "Error retrieving manifest", http.StatusInternalServerError)
		return
	}

	fileBlocks := make(map[int][]models.ChunkRequest)
	shardsToRetrieve := fileManifest.ReedSolomonConfig.DataShards

	for blockIndex, blockData := range fileManifest.Split {
		log.Printf("[GetFile] - Retrieving chunks for block %d...\n", blockIndex)
		fileBlocks[blockIndex] = []models.ChunkRequest{}
		shardsRetrieved := 0

		for _, chunkInfo := range blockData.Chunks {
			if shardsRetrieved >= shardsToRetrieve {
				break
			}

			for _, node := range chunkInfo.Nodes {
				chunk, err := service.RequestChunk(node, chunkInfo.ChunkId)
				if err != nil {
					continue
				}

				fileBlocks[blockIndex] = append(fileBlocks[blockIndex], *chunk)
				shardsRetrieved++

				log.Printf("[GetFile] - Retrieved chunk %s (ShardIndex %d) for block %d from %s successfully\n", chunkInfo.ChunkId, chunkInfo.ShardIndex, blockIndex, node)
				break
			}

		}

		if shardsRetrieved < shardsToRetrieve {
			log.Printf("[GetFile] - Error: Insufficient data for block %d. Required %d, got %d\n", blockIndex, shardsToRetrieve, shardsRetrieved)
			http.Error(w, fmt.Sprintf("Insufficient data to reconstruct file (block %d)", blockIndex), http.StatusUnauthorized)
			return
		}
	}

	log.Println("[GetFile] - All necessary chunks retrieved. Starting local file reconstruction...")

	savedFilePath, err := service.ReconstructAndSaveFileLocal(fileManifest, fileBlocks, config.FileGetDestDir)
	if err != nil {
		log.Printf("[GetFile] - Error during file reconstruction: %v\n", err)
		http.Error(w, "Error during file reconstruction", http.StatusInternalServerError)
		return
	}

	file, err := os.Open(savedFilePath)
	if err != nil {
		log.Printf("[GetFile] - Error during opening the file: %v\n", err)
		http.Error(w, "Error during file opening", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		log.Printf("[GetFile] - Error during calculate the hash of the file: %v\n", err)
		http.Error(w, "Error during file hash calculation", http.StatusInternalServerError)
		return
	}
	hashFile := hex.EncodeToString(hasher.Sum(nil))
	if hashFile != fileManifest.HashFile {
		log.Printf("[GetFile] - The hash of the reconstructed file is diffrent from the original. Probably is corrupted")
		http.Error(w, "The hash of the reconstructed file is diffrent from the original. Probably is corrupted", http.StatusInternalServerError)
		return
	}
	log.Printf("[GetFile] - File successfully reconstructed and saved to: %s\n", savedFilePath)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "File downloaded and reconstructed successfully.",
		"filePath": savedFilePath,
		"fileId":   fileId,
	})
}
