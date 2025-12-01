package config

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/boltdb/bolt"
)

var (
	OnionAddress string
	HttpClient   *http.Client
	PublicKey    []byte
	PrivateKey   []byte
	BoltDB       *bolt.DB

	CronClean        int      = 3600
	Port             int      = 8081
	SocksPort        int      = 9050
	BootStrapServers []string = []string{}
	TargetChunkSize           = 500 * 1024
	DataShards                = 3
	ParityShards              = 2
	TotalShards               = DataShards + ParityShards
	ChunksTolerance           = 3
	DatabaseService           = "BoltDB"

	TorPath        string
	TorDataDir     string
	PrivateKeyDir  string
	PublicKeyDir   string
	FileGetDestDir string
	DrandChainHash = "52db9ba70e0cc0f6eaf7803dd07447a1f5477735fd3f661792ba94600c84e971"
	DrandRelays    = []string{"https://api.drand.sh", "https://drand.cloudflare.com"}
)

func InitConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	baseDir := filepath.Join(home, ".kairos", "client")

	TorPath = filepath.Join("..", "..", "internal", "tor", "tor-bundle-default", "tor", "tor")
	TorDataDir = filepath.Join("..", "..", "internal", "tor", "tor-bundle-default", "tor_data")
	PrivateKeyDir = filepath.Join(baseDir, "keys", "private_key.pem")
	PublicKeyDir = filepath.Join(baseDir, "keys", "public_key.pem")

	FileGetDestDir = filepath.Join(home, "Downloads")

	return nil
}
