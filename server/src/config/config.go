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
	Port             int      = 3000
	SocksPort        int      = 9051
	BootStrapServers []string = []string{"rtet7prci243bl7eo7bufw535sf7mzmbaggwy5abjxwrskqa6263xpqd.onion:3000"} // add more bootstrap servers
	CronSync         int      = 10
	MaxNodesReturned int      = 50
	DatabaseService           = "BoltDB"
	TorPath                   = "../tor/tor-bundle-default/tor/tor"
	TorDataDir                = "../tor/tor-bundle-default/data"
	PrivateKeyDir             = "../keys/private_key.pem"
	PublicKeyDir              = "../keys/public_key.pem"
)
