package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/FraMan97/kairos/client/src/config"
	"github.com/FraMan97/kairos/client/src/model"
	"github.com/corvus-ch/shamir"
	"github.com/google/uuid"
	"github.com/klauspost/reedsolomon"
)

func SplitFile(file multipart.File, blockSize int) (map[int]map[string][][]byte, map[int]int, error) {
	enc, _ := reedsolomon.New(config.DataShards, config.ParityShards)

	buffer := make([]byte, blockSize)
	blockID := 0
	results := make(map[int]map[string][][]byte)
	blockSizes := make(map[int]int)
	for {
		n, err := io.ReadFull(file, buffer)
		if err == io.EOF {
			break
		}

		if err == io.ErrUnexpectedEOF {
			buffer = buffer[:n]
			err = nil
		} else if err != nil {
			return nil, nil, err
		}

		key := GenerateRandomAESKey()

		encryptedBlock, err := EncryptGCM(buffer, key)
		if err != nil {
			return nil, nil, err
		}

		blockSizes[blockID] = len(encryptedBlock)

		dataChunks, err := enc.Split(encryptedBlock)
		if err != nil {
			return nil, nil, err
		}

		err = enc.Encode(dataChunks)
		if err != nil {
			return nil, nil, err
		}

		keyParts, err := shamir.Split(key, config.TotalShards, config.DataShards)
		if err != nil {
			return nil, nil, err
		}
		var shamirIndexes []byte
		for k := range keyParts {
			shamirIndexes = append(shamirIndexes, k)
		}
		results[blockID] = make(map[string][][]byte)
		for i := 0; i < config.TotalShards; i++ {
			payloadDati := dataChunks[i]

			currentIndex := shamirIndexes[i]
			rawKeyPart := keyParts[currentIndex]

			finalKeyPayload := append([]byte{currentIndex}, rawKeyPart...)

			dataSafe := make([]byte, len(payloadDati))
			copy(dataSafe, payloadDati)

			results[blockID]["key"] = append(results[blockID]["key"], finalKeyPayload)
			results[blockID]["data"] = append(results[blockID]["data"], dataSafe)
		}

		blockID++
		if n < blockSize {
			break
		}
	}
	return results, blockSizes, nil
}

func GenerateFileManifest(mapping map[int]map[string][][]byte, blockSizes map[int]int, nodes []string, file multipart.File, header *multipart.FileHeader, releaseTime string) (*model.FileManifest, error) {
	log.Printf("[FileManagement] - Generating file manifest...")

	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}

	fileHash := hex.EncodeToString(hasher.Sum(nil))

	var fileManifest model.FileManifest
	fileManifest.FileName = header.Filename
	fileManifest.FileSize = header.Size
	fileManifest.ReleaseDate = releaseTime
	fileManifest.FileId = uuid.New().String()
	fileManifest.HashFile = fileHash
	fileManifest.HashAlgorithm = "SHA256"
	fileManifest.Blocks = len(mapping)
	fileManifest.ChunksPerBlocks = config.TotalShards
	fileManifest.ReedSolomonConfig = model.ReedSolomonConfig{DataShards: config.DataShards, ParityShards: config.ParityShards}
	fileManifest.Split = make(map[int]model.FileBlock)
	for i := 0; i < len(mapping); i++ {
		fileBlock := model.FileBlock{
			EncryptedBlockSize: blockSizes[i],
			Chunks:             make([]model.Chunk, 0, config.TotalShards),
		}

		for j := 0; j < len(mapping[i]["key"]); j++ {
			keyPayload := mapping[i]["key"][j]

			shamirIndex := keyPayload[0]
			shamirPart := keyPayload[1:]

			selectedNodes := pickRandomItems(nodes, config.ChunksTolerance)
			chunk := model.Chunk{
				ShardIndex:   j,
				KeyIndexPart: shamirIndex,
				KeyPart:      shamirPart,
				Nodes:        selectedNodes,
				ChunkId:      uuid.New().String(),
			}

			fileBlock.Chunks = append(fileBlock.Chunks, chunk)
		}
		fileManifest.Split[i] = fileBlock
	}

	log.Printf("[FileManagement] - File manifest generated successfully")
	return &fileManifest, nil
}

func RequestNodesForFileUpload(totalChunks int) ([]string, error) {
	chosenServer := rand.Intn(len(config.BootStrapServers))
	log.Printf("[FileManagement] - Chosen Bootstrap Server %s to request nodes", config.BootStrapServers[chosenServer])

	request := model.NodesForFileUploadRequest{
		Address:       config.OnionAddress + ":" + strconv.Itoa(config.Port),
		PublicKey:     config.PublicKey,
		TotalChunks:   totalChunks,
		NodesPerChunk: config.ChunksTolerance,
	}

	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	signature, err := SignMessage(jsonBytes)
	if err != nil {
		return nil, err
	}
	request.Signature = signature

	jsonBytes, err = json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/file/nodes", config.BootStrapServers[chosenServer]),
		"application/json",
		bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		log.Printf("[FileManagement] - Nodes got successfully from http://%s!\n", config.BootStrapServers[chosenServer])
		var response []string
		json.NewDecoder(resp.Body).Decode(&response)
		return response, nil
	} else {
		return nil, fmt.Errorf("error with message: %v", resp.Body)

	}

}

func UploadFileManifest(fileManifest *model.FileManifest) error {
	chosenServer := rand.Intn(len(config.BootStrapServers))
	log.Printf("[FileManagement] - Chosen Bootstrap Server %s to upload the file manifest", config.BootStrapServers[chosenServer])

	manifestBytes, err := json.Marshal(*fileManifest)
	if err != nil {
		return err
	}

	hashToSign := sha256.Sum256(manifestBytes)

	signature, err := SignMessage(hashToSign[:])
	if err != nil {
		return err
	}

	fileManifestRequest := model.FileManifestRequest{
		Address:   config.OnionAddress + ":" + strconv.Itoa(config.Port),
		PublicKey: config.PublicKey,
		Manifest:  *fileManifest,
		Signature: signature,
	}

	jsonBytesToSend, err := json.Marshal(fileManifestRequest)
	if err != nil {
		return err
	}

	resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/file/manifest", config.BootStrapServers[chosenServer]),
		"application/json",
		bytes.NewBuffer(jsonBytesToSend))
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		log.Printf("[FileManagement] - File manifest upload to Bootstrap Server %s", config.BootStrapServers[chosenServer])
		return nil
	} else {
		return fmt.Errorf("error with message: %v", resp.Body)
	}
}

func UploadFile(fileManifest *model.FileManifest, mapping map[int]map[string][][]byte) error {
	log.Printf("[FileManagement] - Uploading File %s to nodes...", fileManifest.FileId)

	chunkRequest := model.ChunkRequest{
		Address:   config.OnionAddress + ":" + strconv.Itoa(config.Port),
		PublicKey: config.PublicKey,
	}
	for i := 0; i < len(fileManifest.Split); i++ {
		block := fileManifest.Split[i]
		almostOneChunk := false
		for j := 0; j < len(block.Chunks); j++ {
			chunkRequest.Signature = nil
			chunk := block.Chunks[j]
			chunkRequest.ChunkId = chunk.ChunkId
			chunkRequest.ReleaseDate = fileManifest.ReleaseDate

			dataChunk := mapping[i]["data"][j]
			chunkRequest.Shard = dataChunk

			jsonBytes, err := json.Marshal(chunkRequest)
			if err != nil {
				continue
			}

			signature, err := SignMessage(jsonBytes)
			if err != nil {
				continue
			}
			chunkRequest.Signature = signature

			jsonBytes, err = json.Marshal(chunkRequest)
			if err != nil {
				continue
			}
			for _, v := range chunk.Nodes {

				resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/chunk", v),
					"application/json",
					bytes.NewBuffer(jsonBytes))
				if err != nil {
					continue
				}

				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					log.Printf("[FileManagement] - Upload chunk %s to http://%s successfully!\n", chunk.ChunkId, v)
					almostOneChunk = true

				} else {
					continue
				}
			}
			if !almostOneChunk {
				break
			}
		}
		if !almostOneChunk {
			log.Printf("[FileManagement] - One block could not be sent because at least one chunk was not successfully sent to any node\n")
			return fmt.Errorf("one block could not be sent because at least one chunk was not successfully sent to any node")
		}
	}
	return nil
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

func GetFileManifestFromServer(fileId string) (*model.FileManifest, error) {
	chosenServer := rand.Intn(len(config.BootStrapServers))
	log.Printf("[FileManagement] - Chosen Bootstrap Server %s to get the file manifest", config.BootStrapServers[chosenServer])
	request := model.GetFileManifestRequest{
		Address:   config.OnionAddress + ":" + strconv.Itoa(config.Port),
		PublicKey: config.PublicKey,
		FileId:    fileId,
	}

	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	signature, err := SignMessage(jsonBytes)
	if err != nil {
		return nil, err
	}
	request.Signature = signature

	jsonBytes, err = json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/manifests", config.BootStrapServers[chosenServer]),
		"application/json",
		bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		log.Printf("[FileManagement] - Got file manifest successfully from http://%s!\n", config.BootStrapServers[chosenServer])
		var response *model.FileManifest
		json.NewDecoder(resp.Body).Decode(&response)
		return response, nil
	} else {
		return nil, fmt.Errorf("error with message: %v", resp.Body)

	}

}

func SaveChunk(r *http.Request) error {
	log.Println("[FileManagement] - Saving chunk in DB...")
	var chunkRequest model.ChunkRequest

	err := json.NewDecoder(r.Body).Decode(&chunkRequest)
	if err != nil {
		return err
	}

	defer r.Body.Close()

	message, err := json.Marshal(model.ChunkRequest{Address: chunkRequest.Address, PublicKey: chunkRequest.PublicKey,
		ChunkId: chunkRequest.ChunkId, Shard: chunkRequest.Shard, ReleaseDate: chunkRequest.ReleaseDate})
	if err != nil {
		return err
	}

	check, err := VerifySignature(message, chunkRequest.Signature, chunkRequest.PublicKey)
	if err != nil {
		return err
	}
	if check {
		payload, err := json.Marshal(model.ChunkRequest{PublicKey: chunkRequest.PublicKey, Address: chunkRequest.Address,
			ChunkId: chunkRequest.ChunkId, Shard: chunkRequest.Shard, ReleaseDate: chunkRequest.ReleaseDate})
		if err != nil {
			return err
		}
		err = PutData(config.BoltDB, "chunks", chunkRequest.ChunkId, payload)
		if err != nil {
			return err
		}
		log.Println("[FileManagement] - Saved chunk in DB successfully")
	} else {
		return err
	}
	return nil
}

func GetChunk(r *http.Request) ([]byte, error) {
	log.Println("[FileManagement] - Getting chunk from DB...")
	chunkId := r.URL.Query().Get("chunkId")
	if chunkId == "" {
		return nil, fmt.Errorf("error 'chunkId' empty")
	}

	var chunkRequest model.ChunkRequest

	defer r.Body.Close()

	chunk, err := GetData(config.BoltDB, "chunks", chunkId)
	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(bytes.NewBuffer(chunk)).Decode(&chunkRequest)
	if err != nil {
		return nil, err
	}

	parsedTime, err := time.Parse(time.RFC3339, chunkRequest.ReleaseDate)
	if err != nil {
		return nil, err
	}

	currentTime := time.Now().UTC()
	if !currentTime.After(parsedTime) {
		log.Printf("[FileManagement] - Chunk %s access denied: current time %s is NOT after release time %s", chunkId, currentTime, parsedTime)
		return nil, fmt.Errorf("the release date is not already expired (%s)", parsedTime)
	} else {
		log.Printf("[FileManagement] - Chunk %s access granted: current time %s is after release time %s", chunkId, currentTime, parsedTime)
		return chunk, nil
	}
}

func ReconstructAndSaveFileLocal(fileManifest *model.FileManifest, fileBlocks map[int][]model.ChunkRequest, destinationFolder string) (string, error) {
	log.Println("[FileManagement] - Reconstructing...")
	err := os.MkdirAll(destinationFolder, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create destination folder: %v", err)
	}

	filePath := filepath.Join(destinationFolder, fileManifest.FileName)
	outFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	enc, err := reedsolomon.New(fileManifest.ReedSolomonConfig.DataShards, fileManifest.ReedSolomonConfig.ParityShards)
	if err != nil {
		return "", fmt.Errorf("Reed-Solomon creation error: %v", err)
	}

	totalShards := fileManifest.ReedSolomonConfig.DataShards + fileManifest.ReedSolomonConfig.ParityShards

	for i := 0; i < fileManifest.Blocks; i++ {
		blockChunks, ok := fileBlocks[i]
		if !ok {
			return "", fmt.Errorf("missing data for block %d", i)
		}

		originalEncryptedSize := fileManifest.Split[i].EncryptedBlockSize
		if originalEncryptedSize == 0 {
			return "", fmt.Errorf("missing encrypted block size for block %d in manifest", i)
		}

		shards := make([][]byte, totalShards)
		keyParts := make(map[byte][]byte)
		shardsReceived := 0

		originalChunkInfo := make(map[string]model.Chunk)
		for _, c := range fileManifest.Split[i].Chunks {
			originalChunkInfo[c.ChunkId] = c
		}

		for _, chunkReq := range blockChunks {
			originalInfo, infoFound := originalChunkInfo[chunkReq.ChunkId]
			if !infoFound {
				log.Printf("Warning: received chunkId '%s' not in manifest for block %d", chunkReq.ChunkId, i)
				continue
			}

			if shards[originalInfo.ShardIndex] != nil {
				continue
			}

			shards[originalInfo.ShardIndex] = chunkReq.Shard
			keyParts[originalInfo.KeyIndexPart] = originalInfo.KeyPart
			shardsReceived++
		}

		if shardsReceived < fileManifest.ReedSolomonConfig.DataShards {
			return "", fmt.Errorf("insufficient data to reconstruct block %d: required %d, received %d", i, fileManifest.ReedSolomonConfig.DataShards, shardsReceived)
		}

		err = enc.Reconstruct(shards)
		if err != nil {
			return "", fmt.Errorf("reconstruction failed for block %d: %v", i, err)
		}

		var encryptedBlock bytes.Buffer
		err = enc.Join(&encryptedBlock, shards, originalEncryptedSize)
		if err != nil {
			return "", fmt.Errorf("failed to join shards for block %d: %v", i, err)
		}

		if len(keyParts) < fileManifest.ReedSolomonConfig.DataShards {
			return "", fmt.Errorf("insufficient key parts for block %d: required %d, received %d", i, fileManifest.ReedSolomonConfig.DataShards, len(keyParts))
		}

		aesKey, err := shamir.Combine(keyParts)
		if err != nil {
			return "", fmt.Errorf("failed to combine Shamir key for block %d: %v", i, err)
		}

		decryptedBlock, err := DecryptGCM(encryptedBlock.Bytes(), aesKey)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt block %d: %v", i, err)
		}

		_, err = outFile.Write(decryptedBlock)
		if err != nil {
			return "", fmt.Errorf("failed to write block %d to file: %v", i, err)
		}
	}

	return filePath, nil
}

func RequestChunk(node string, chunkId string) (*model.ChunkRequest, error) {

	resp, err := config.HttpClient.Get(fmt.Sprintf("http://%s/chunk?chunkId=%s", node, chunkId))

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var chunkRequest model.ChunkRequest
		log.Printf("[FileManagement] - Get chunk %s from http://%s!\n", chunkId, node)
		err := json.NewDecoder(resp.Body).Decode(&chunkRequest)
		if err != nil {
			log.Printf("Error decoding chunk response from %s: %v", node, err)
			return nil, err
		}
		return &chunkRequest, nil
	} else {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error with message: %s", string(body))
	}
}
