package model

type SubscriptionRequest struct {
	Address   string `json:"address"`
	PublicKey []byte `json:"public_key"`
	Signature []byte `json:"signature"`
}

type FileManifestRequest struct {
	Address   string       `json:"address"`
	PublicKey []byte       `json:"public_key"`
	Signature []byte       `json:"signature"`
	Manifest  FileManifest `json:"manifest"`
}

type GetFileManifestRequest struct {
	Address   string `json:"address"`
	PublicKey []byte `json:"public_key"`
	Signature []byte `json:"signature"`
	FileId    string `json:"file_id"`
}

type NodesForFileUploadRequest struct {
	Address       string `json:"address"`
	PublicKey     []byte `json:"public_key"`
	Signature     []byte `json:"signature"`
	TotalChunks   int    `json:"total_chunks"`
	NodesPerChunk int    `json:"nodes_per_chunk"`
}

type ChunkRequest struct {
	Address     string `json:"address"`
	PublicKey   []byte `json:"public_key"`
	Signature   []byte `json:"signature"`
	ChunkId     string `json:"chunk_id"`
	Shard       []byte `json:"shard"`
	ReleaseDate string `json:"release_date"`
}

type FileManifest struct {
	FileName          string
	FileId            string
	FileSize          int64
	ReleaseDate       string
	HashFile          string
	HashAlgorithm     string
	Blocks            int
	ChunksPerBlocks   int
	ReedSolomonConfig ReedSolomonConfig
	Split             map[int]FileBlock
}

type ReedSolomonConfig struct {
	DataShards   int
	ParityShards int
}

type FileBlock struct {
	EncryptedBlockSize int
	Chunks             []Chunk
}

type Chunk struct {
	ChunkId      string
	KeyIndexPart byte
	KeyPart      []byte
	ShardIndex   int
	Nodes        []string
}
