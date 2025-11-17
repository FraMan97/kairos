package service

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/FraMan97/kairos/client/src/config"
	"golang.org/x/net/proxy"
)

func StartTor() (string, error) {
	log.Println("[Tor] - Starting Tor...")

	hiddenServiceDir := filepath.Join(config.TorDataDir, "hidden_service")

	os.MkdirAll(config.TorDataDir, 0700)
	os.MkdirAll(hiddenServiceDir, 0700)

	cmd := exec.Command(config.TorPath,
		"--DataDirectory", config.TorDataDir,
		"--HiddenServiceDir", hiddenServiceDir,
		"--SocksPort", strconv.Itoa(config.SocksPort),
		"--HiddenServicePort", fmt.Sprintf("%d 127.0.0.1:%d", config.Port, config.Port),
	)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		log.Println("[Tor] - Error starting Tor:", err)
		return "", fmt.Errorf("error starting Tor: %w", err)
	}

	bootstrapped := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			if strings.Contains(line, "Bootstrapped 100%") {
				bootstrapped <- true
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Println(scanner.Text())
		}
	}()

	select {
	case <-bootstrapped:
		log.Println("[Tor] - Tor bootstrapped!")
	case <-time.After(3 * time.Minute):
		cmd.Process.Kill()
		return "", fmt.Errorf("tor bootstrap timeout")
	}

	time.Sleep(2 * time.Second)

	hostnameFile := filepath.Join(hiddenServiceDir, "hostname")
	onionAddr, err := os.ReadFile(hostnameFile)
	if err != nil {
		log.Printf("[Tor] - Error reading .onion address from %s : %v\n", hostnameFile, err)
		return "", fmt.Errorf("failed to read onion address: %w", err)
	}

	addr := strings.TrimSpace(string(onionAddr))
	log.Println("[Tor] - Hidden service created!")
	log.Printf("[Tor] - .onion address: http://%s\n", addr)
	config.OnionAddress = addr
	config.HttpClient, err = createClientTor()
	if err != nil {
		log.Println("[Tor] - Error creating http client:", err)
		return "", fmt.Errorf("error creating http client: %w", err)
	}
	return addr, nil
}

func createClientTor() (*http.Client, error) {
	torProxy, err := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%s", strconv.Itoa(config.SocksPort)))
	if err != nil {
		return nil, err
	}

	dialer, err := proxy.FromURL(torProxy, proxy.Direct)
	if err != nil {
		return nil, err
	}

	httpTransport := &http.Transport{
		Dial: dialer.Dial,
	}

	httpClient := &http.Client{
		Transport: httpTransport,
	}

	return httpClient, nil
}
