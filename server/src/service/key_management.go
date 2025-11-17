package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"

	"github.com/FraMan97/kairos/server/src/config"
)

func GenerateKeyPair() error {
	keysDir := "../keys"
	os.MkdirAll(keysDir, 0700)

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("error generating keys: %w", err)
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("error marshaling private key: %w", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	err = os.WriteFile("../keys/private_key.pem", privateKeyPEM, 0600)
	if err != nil {
		return fmt.Errorf("error saving private key: %w", err)
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("error marshaling public key: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	err = os.WriteFile("../keys/public_key.pem", publicKeyPEM, 0644)
	if err != nil {
		return fmt.Errorf("error saving public key: %w", err)
	}

	log.Println("Keys generated successfully!")
	log.Println("Private key: private_key.pem")
	log.Println("Public key: public_key.pem")

	publicKey, err = GetPublicKey()
	if err != nil {
		return err
	}
	config.PublicKey = publicKey
	privateKey, err = GetPrivateKey()
	if err != nil {
		return err
	}
	config.PrivateKey = privateKey
	return nil
}

func SignMessage(message []byte) ([]byte, error) {
	privateKeyPEM, err := os.ReadFile("../keys/private_key.pem")
	if err != nil {
		return nil, fmt.Errorf("error reading private key: %w", err)
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKeyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing private key: %w", err)
	}

	privateKey, ok := privateKeyInterface.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 private key")
	}

	signature := ed25519.Sign(privateKey, message)

	return signature, nil
}

func GetPublicKey() ([]byte, error) {
	privateKeyPEM, err := os.ReadFile("../keys/public_key.pem")
	if err != nil {
		return nil, fmt.Errorf("error reading public key: %w", err)
	}

	return privateKeyPEM, nil
}

func GetPrivateKey() ([]byte, error) {
	privateKeyPEM, err := os.ReadFile("../keys/private_key.pem")
	if err != nil {
		return nil, fmt.Errorf("error reading private key: %w", err)
	}

	return privateKeyPEM, nil
}

func VerifySignature(message []byte, signature []byte, publicKey []byte) (bool, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return false, fmt.Errorf("failed to decode PEM block")
	}

	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("error parsing public key: %w", err)
	}

	publicKey, ok := publicKeyInterface.(ed25519.PublicKey)
	if !ok {
		return false, fmt.Errorf("not an Ed25519 public key")
	}

	return ed25519.Verify(publicKey, message, signature), nil
}
