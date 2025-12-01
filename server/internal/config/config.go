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

	Port             int      = 3000
	SocksPort        int      = 9051
	BootStrapServers []string = []string{}
	CronSync         int      = 10
	CronClean        int      = 3600
	MaxNodesReturned int      = 50
	DatabaseService           = "BoltDB"

	TorPath       string
	TorDataDir    string
	PrivateKeyDir string
	PublicKeyDir  string
)

func InitConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	baseDir := filepath.Join(home, ".kairos", "server")

	TorPath = filepath.Join("..", "..", "internal", "tor", "tor-bundle-default", "tor", "tor")
	TorDataDir = filepath.Join("..", "..", "internal", "tor", "tor-bundle-default", "tor_data")
	PrivateKeyDir = filepath.Join(baseDir, "keys", "private_key.pem")
	PublicKeyDir = filepath.Join(baseDir, "keys", "public_key.pem")

	return nil
}
