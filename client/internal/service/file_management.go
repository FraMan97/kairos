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

	"github.com/FraMan97/kairos/client/internal/config"
	"github.com/FraMan97/kairos/client/internal/crypto"
	"github.com/FraMan97/kairos/client/internal/database"
	"github.com/FraMan97/kairos/client/internal/models"
	"github.com/corvus-ch/shamir"
	"github.com/google/uuid"
	"github.com/klauspost/reedsolomon"

	"github.com/drand/tlock"
	tlock_http "github.com/drand/tlock/networks/http"
)

func SplitFile(file multipart.File, blockSize int, releaseTime string) (map[int]map[string][][]byte, map[int]int, error) {
	drandRound, err := GetRoundForTime(releaseTime)
	if err != nil {
		return nil, nil, err
	}
	log.Printf("[Drand] - Encryption Time-Lock for the round: %d\n", drandRound)

	tNetwork, err := tlock_http.NewNetwork(config.DrandRelays[rand.Intn(len(config.DrandRelays))], config.DrandChainHash)
	if err != nil {
		return nil, nil, fmt.Errorf("errore network tlock: %v", err)
	}
	tlockClient := tlock.New(tNetwork)

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

		key := crypto.GenerateRandomAESKey()

		var encryptedKeyBuf bytes.Buffer
		err = tlockClient.Encrypt(&encryptedKeyBuf, bytes.NewReader(key), drandRound)
		if err != nil {
			return nil, nil, err
		}
		encryptedKey := encryptedKeyBuf.Bytes()

		encryptedBlock, err := crypto.EncryptGCM(buffer, key)
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

		keyParts, err := shamir.Split(encryptedKey, config.TotalShards, config.DataShards)
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

func ReconstructAndSaveFileLocal(fileManifest *models.FileManifest, fileBlocks map[int][]models.ChunkRequest, destinationFolder string) (string, error) {
	log.Println("[FileManagement] - Reconstructing...")
	err := os.MkdirAll(destinationFolder, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create destination folder: %v", err)
	}

	tNetwork, err := tlock_http.NewNetwork(config.DrandRelays[rand.Intn(len(config.DrandRelays))], config.DrandChainHash)
	if err != nil {
		return "", fmt.Errorf("errore network tlock: %v", err)
	}
	tlockClient := tlock.New(tNetwork)

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
		shards := make([][]byte, totalShards)
		keyParts := make(map[byte][]byte)
		shardsReceived := 0
		originalChunkInfo := make(map[string]models.Chunk)
		for _, c := range fileManifest.Split[i].Chunks {
			originalChunkInfo[c.ChunkId] = c
		}
		for _, chunkReq := range blockChunks {
			originalInfo, infoFound := originalChunkInfo[chunkReq.ChunkId]
			if !infoFound || shards[originalInfo.ShardIndex] != nil {
				continue
			}
			shards[originalInfo.ShardIndex] = chunkReq.Shard
			keyParts[originalInfo.KeyIndexPart] = originalInfo.KeyPart
			shardsReceived++
		}
		if shardsReceived < fileManifest.ReedSolomonConfig.DataShards {
			return "", fmt.Errorf("insufficient data block %d", i)
		}

		err = enc.Reconstruct(shards)
		if err != nil {
			return "", fmt.Errorf("reconstruction failed for block %d: %v", i, err)
		}
		var encryptedBlock bytes.Buffer
		err = enc.Join(&encryptedBlock, shards, originalEncryptedSize)
		if err != nil {
			return "", fmt.Errorf("failed to join shards: %v", err)
		}

		encryptedAESKey, err := shamir.Combine(keyParts)
		if err != nil {
			return "", fmt.Errorf("failed to combine Shamir key: %v", err)
		}

		var plainAESKeyBuf bytes.Buffer
		err = tlockClient.Decrypt(&plainAESKeyBuf, bytes.NewReader(encryptedAESKey))
		if err != nil {
			return "", fmt.Errorf("failed to unlock the key with drand: %v", err)
		}
		plainAESKey := plainAESKeyBuf.Bytes()

		decryptedBlock, err := crypto.DecryptGCM(encryptedBlock.Bytes(), plainAESKey)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt block AES: %v", err)
		}

		_, err = outFile.Write(decryptedBlock)
		if err != nil {
			return "", fmt.Errorf("failed to write block: %v", err)
		}
	}

	return filePath, nil
}

func GetRoundForTime(releaseTimeStr string) (uint64, error) {
	targetTime, err := time.Parse(time.RFC3339, releaseTimeStr)
	if err != nil {
		return 0, err
	}

	net, err := tlock_http.NewNetwork(config.DrandRelays[rand.Intn(len(config.DrandRelays))], config.DrandChainHash)
	if err != nil {
		return 0, err
	}

	round := net.Current(targetTime)

	if round == 0 {
		return 0, err
	}

	return round, nil
}

func GenerateFileManifest(mapping map[int]map[string][][]byte, blockSizes map[int]int, nodes []string, file multipart.File, header *multipart.FileHeader, releaseTime string) (*models.FileManifest, error) {
	log.Printf("[FileManagement] - Generating file manifest...")
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	fileHash := hex.EncodeToString(hasher.Sum(nil))
	var fileManifest models.FileManifest
	fileManifest.FileName = header.Filename
	fileManifest.FileSize = header.Size
	fileManifest.ReleaseDate = releaseTime
	fileManifest.FileId = uuid.New().String()
	fileManifest.HashFile = fileHash
	fileManifest.HashAlgorithm = "SHA256"
	fileManifest.Blocks = len(mapping)
	fileManifest.ChunksPerBlocks = config.TotalShards
	fileManifest.ReedSolomonConfig = models.ReedSolomonConfig{DataShards: config.DataShards, ParityShards: config.ParityShards}
	fileManifest.Split = make(map[int]models.FileBlock)
	for i := 0; i < len(mapping); i++ {
		fileBlock := models.FileBlock{EncryptedBlockSize: blockSizes[i], Chunks: make([]models.Chunk, 0, config.TotalShards)}
		for j := 0; j < len(mapping[i]["key"]); j++ {
			keyPayload := mapping[i]["key"][j]
			shamirIndex := keyPayload[0]
			shamirPart := keyPayload[1:]
			selectedNodes := pickRandomItems(nodes, config.ChunksTolerance)
			chunk := models.Chunk{ShardIndex: j, KeyIndexPart: shamirIndex, KeyPart: shamirPart, Nodes: selectedNodes, ChunkId: uuid.New().String()}
			fileBlock.Chunks = append(fileBlock.Chunks, chunk)
		}
		fileManifest.Split[i] = fileBlock
	}
	return &fileManifest, nil
}

func RequestNodesForFileUpload(totalChunks int) ([]string, error) {
	chosenServer := rand.Intn(len(config.BootStrapServers))
	request := models.NodesForFileUploadRequest{Address: config.OnionAddress + ":" + strconv.Itoa(config.Port), PublicKey: config.PublicKey, TotalChunks: totalChunks, NodesPerChunk: config.ChunksTolerance}
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	signature, err := crypto.SignMessage(jsonBytes)
	if err != nil {
		return nil, err
	}
	request.Signature = signature
	jsonBytes, err = json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/file/nodes", config.BootStrapServers[chosenServer]), "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var response []string
		json.NewDecoder(resp.Body).Decode(&response)
		return response, nil
	} else {
		return nil, fmt.Errorf("error with message: %v", resp.Body)
	}
}

func UploadFileManifest(fileManifest *models.FileManifest) error {
	chosenServer := rand.Intn(len(config.BootStrapServers))
	manifestBytes, err := json.Marshal(*fileManifest)
	if err != nil {
		return err
	}
	hashToSign := sha256.Sum256(manifestBytes)
	signature, err := crypto.SignMessage(hashToSign[:])
	if err != nil {
		return err
	}
	fileManifestRequest := models.FileManifestRequest{Address: config.OnionAddress + ":" + strconv.Itoa(config.Port), PublicKey: config.PublicKey, Manifest: *fileManifest, Signature: signature}
	jsonBytesToSend, err := json.Marshal(fileManifestRequest)
	if err != nil {
		return err
	}
	resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/file/manifest", config.BootStrapServers[chosenServer]), "application/json", bytes.NewBuffer(jsonBytesToSend))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil
	} else {
		return fmt.Errorf("error with message: %v", resp.Body)
	}
}

func UploadFile(fileManifest *models.FileManifest, mapping map[int]map[string][][]byte) error {
	log.Printf("[FileManagement] - Uploading File %s to nodes...", fileManifest.FileId)
	chunkRequest := models.ChunkRequest{Address: config.OnionAddress + ":" + strconv.Itoa(config.Port), PublicKey: config.PublicKey}
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
			signature, err := crypto.SignMessage(jsonBytes)
			if err != nil {
				continue
			}
			chunkRequest.Signature = signature
			jsonBytes, err = json.Marshal(chunkRequest)
			if err != nil {
				continue
			}
			for _, v := range chunk.Nodes {
				resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/chunk", v), "application/json", bytes.NewBuffer(jsonBytes))
				if err != nil {
					continue
				}
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
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
			return fmt.Errorf("one block could not be sent")
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

func GetFileManifestFromServer(fileId string) (*models.FileManifest, error) {
	chosenServer := rand.Intn(len(config.BootStrapServers))
	request := models.GetFileManifestRequest{Address: config.OnionAddress + ":" + strconv.Itoa(config.Port), PublicKey: config.PublicKey, FileId: fileId}
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	signature, err := crypto.SignMessage(jsonBytes)
	if err != nil {
		return nil, err
	}
	request.Signature = signature
	jsonBytes, err = json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/manifests", config.BootStrapServers[chosenServer]), "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var response *models.FileManifest
		json.NewDecoder(resp.Body).Decode(&response)
		return response, nil
	} else {
		return nil, fmt.Errorf("error with message: %v", resp.Body)
	}
}

func SaveChunk(r *http.Request) error {
	var chunkRequest models.ChunkRequest
	err := json.NewDecoder(r.Body).Decode(&chunkRequest)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	message, err := json.Marshal(models.ChunkRequest{Address: chunkRequest.Address, PublicKey: chunkRequest.PublicKey, ChunkId: chunkRequest.ChunkId, Shard: chunkRequest.Shard, ReleaseDate: chunkRequest.ReleaseDate})
	if err != nil {
		return err
	}
	check, err := crypto.VerifySignature(message, chunkRequest.Signature, chunkRequest.PublicKey)
	if err != nil {
		return err
	}
	if check {
		payload, err := json.Marshal(models.ChunkRequest{PublicKey: chunkRequest.PublicKey, Address: chunkRequest.Address, ChunkId: chunkRequest.ChunkId, Shard: chunkRequest.Shard, ReleaseDate: chunkRequest.ReleaseDate})
		if err != nil {
			return err
		}
		err = database.PutData(config.BoltDB, "chunks", chunkRequest.ChunkId, payload)
		if err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

func GetChunk(r *http.Request) ([]byte, error) {
	chunkId := r.URL.Query().Get("chunkId")
	if chunkId == "" {
		return nil, fmt.Errorf("error 'chunkId' empty")
	}
	defer r.Body.Close()
	chunk, err := database.GetData(config.BoltDB, "chunks", chunkId)
	if err != nil {
		return nil, err
	}
	var chunkRequest models.ChunkRequest
	err = json.NewDecoder(bytes.NewBuffer(chunk)).Decode(&chunkRequest)
	if err != nil {
		return nil, err
	}
	return chunk, nil
}

func RequestChunk(node string, chunkId string) (*models.ChunkRequest, error) {
	resp, err := config.HttpClient.Get(fmt.Sprintf("http://%s/chunk?chunkId=%s", node, chunkId))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var chunkRequest models.ChunkRequest
		err := json.NewDecoder(resp.Body).Decode(&chunkRequest)
		if err != nil {
			return nil, err
		}
		return &chunkRequest, nil
	} else {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error: %s", string(body))
	}
}
