package config

import (
	"net/http"

	"github.com/boltdb/bolt"
)

var (
	OnionAddress     string
	HttpClient       *http.Client
	PublicKey        []byte
	PrivateKey       []byte
	BoltDB           *bolt.DB
	CronClean        int      = 3600
	Port             int      = 8081
	SocksPort        int      = 9050
	BootStrapServers []string = []string{"rtet7prci243bl7eo7bufw535sf7mzmbaggwy5abjxwrskqa6263xpqd.onion:3000"} // add more bootstrap servers
	TargetChunkSize           = 500 * 1024
	DataShards                = 3
	ParityShards              = 2
	TotalShards               = DataShards + ParityShards
	ChunksTolerance           = 3
	DatabaseService           = "BoltDB"
	TorPath                   = "../tor/tor-bundle-default/tor/tor"
	TorDataDir                = "../tor/tor-bundle-default/data"
	PrivateKeyDir             = "../keys/private_key.pem"
	PublicKeyDir              = "../keys/public_key.pem"
	FileGetDestDir            = "/home/francesco-mancuso/Downloads" // specify your local path
)
