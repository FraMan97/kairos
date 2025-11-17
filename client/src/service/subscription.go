package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"

	"github.com/FraMan97/kairos/client/src/config"
	"github.com/FraMan97/kairos/client/src/model"
)

func SubscribeNode() error {
	log.Println("[Subscription] - Subscribe Kairos node...")
	chosenServer := rand.Intn(len(config.BootStrapServers))
	subscription := model.SubscriptionRequest{
		Address:   config.OnionAddress + ":" + strconv.Itoa(config.Port),
		PublicKey: config.PublicKey,
	}

	jsonBytes, err := json.Marshal(subscription)
	if err != nil {
		return err
	}

	signature, err := SignMessage(jsonBytes)
	if err != nil {
		return err
	}
	subscription.Signature = signature

	jsonBytes, err = json.Marshal(subscription)
	if err != nil {
		return err
	}
	resp, err := config.HttpClient.Post(fmt.Sprintf("http://%s/subscribe", config.BootStrapServers[chosenServer]),
		"application/json",
		bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		log.Printf("[Subscription] - Node subscribed successfully to http://%s!\n", config.BootStrapServers[chosenServer])
		return nil
	} else {
		return fmt.Errorf("error with message: %v", resp.Body)
	}

}
