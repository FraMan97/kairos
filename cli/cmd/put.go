package cmd

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/FraMan97/kairos/cli/config"
	"github.com/spf13/cobra"
)

var filePath string
var releaseTime string

var putCmd = &cobra.Command{
	Use:   "put",
	Short: "Command to put a file to the network",
	Long: `"Command to send a file to the network, specifying the --file-path argument (the local path where the file is located) 
	and the --release-time argument (which indicates when the file will be made available)"`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Sending file %s ...\n", filePath)
		file, err := os.Open(filePath)
		if err != nil {
			log.Println("Error opening file: ", err)
			return
		}

		body := &bytes.Buffer{}

		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			log.Println("Error creating form file:", err)
			return
		}

		_, err = io.Copy(part, file)
		if err != nil {
			log.Println("Error copying file content:", err)
			return
		}

		err = writer.WriteField("release_time", releaseTime)
		if err != nil {
			log.Println("Error writing release_time field:", err)
			return
		}

		err = writer.Close()
		if err != nil {
			log.Println("Error closing writer:", err)
			return
		}
		resp, err := http.Post(fmt.Sprintf("http://localhost:%s/put", strconv.Itoa(config.Port)),
			writer.FormDataContentType(),
			body)
		if err != nil {
			log.Println("Error calling put endpoint: ", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			log.Printf("File %s sent successfully to the Kairos Network!", filePath)
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("File put successfully in the Kairos Network (status %d), but failed to read response body: %v\n", resp.StatusCode, err)
				return
			}
			log.Printf("File ID: %s\n", string(bodyBytes))
		} else {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error putting file in the Kairos Network (status %d), but failed to read response body: %v\n", resp.StatusCode, err)
				return
			}
			log.Printf("Error putting file in the Kairos Network (status %d): %s\n", resp.StatusCode, string(bodyBytes))
		}
	},
}

func init() {
	rootCmd.AddCommand(putCmd)
	putCmd.Flags().StringVarP(&filePath, "file-path", "f", "", "Path to the file to process")
	putCmd.Flags().StringVarP(&releaseTime, "release-time", "r", "", "Time after publish file (i.e. 2025-12-01T15:00:00Z)")

}
