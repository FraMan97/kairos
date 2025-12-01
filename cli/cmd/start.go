package cmd

import (
	"fmt"
	"io"
	"log"
	"strconv"

	"net/http"

	"github.com/FraMan97/kairos/cli/config"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Command to start the node of Kairos",
	Long:  `"This command starts the node in the Kairos Network by initiating a Tor process, exposing a .onion address, and subscribing to a random Bootstrap Server"`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Kairos Node starting...")
		resp, err := http.Post(fmt.Sprintf("http://localhost:%s/start", strconv.Itoa(config.Port)),
			"application/json",
			nil)
		if err != nil {
			log.Println("Error calling start endpoint: ", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			log.Println("Kairos Node started!")
		} else {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error starting Kairos Node (status %d), but failed to read response body: %v\n", resp.StatusCode, err)
				return
			}
			log.Printf("Error starting Kairos Node (status %d): %s\n", resp.StatusCode, string(bodyBytes))
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

}
