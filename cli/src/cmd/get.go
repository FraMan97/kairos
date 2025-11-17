package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/FraMan97/kairos/cli/src/config"
	"github.com/spf13/cobra"
)

var fileId string

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Command to get a file by fileId",
	Long: `"Command to download a file using the fileId field from the Kairos Network. The client will request the file manifest from a random Bootstrap Server 
	and the list of nodes that hold the chunks. It will then contact the nodes to retrieve them and reconstruct the file"`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Getting file %s from the Kairos Network...", fileId)

		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/get?fileId=%s", strconv.Itoa(config.Port), fileId))
		if err != nil {
			log.Println("Error calling get endpoint: ", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			log.Printf("File %s got and recostructed successfully!\n", fileId)
		} else {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error getting file from the Kairos Network (status %d), but failed to read response body: %v\n", resp.StatusCode, err)
				return
			}
			log.Printf("Error getting file from the Kairos Network (status %d): %s\n", resp.StatusCode, string(bodyBytes))
		}
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().StringVarP(&fileId, "file-id", "f", "", "File id to identify the file")
}
