package controller

import (
	"log"
	"net/http"

	"github.com/FraMan97/kairos/client/src/service"
)

func Chunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		log.Println("[Chunk] - Only POST and GET method allowed!")
		http.Error(w, "Only POST and GET Methods allowed!", http.StatusMethodNotAllowed)
		return
	}

	if r.Method == http.MethodPost {
		err := service.SaveChunk(r)
		if err != nil {
			log.Println("[Chunk] - Error saving chunk in DB: ", err)
			http.Error(w, "Error saving chunk in DB", http.StatusInternalServerError)
			return
		}
	}

	if r.Method == http.MethodGet {
		chunk, err := service.GetChunk(r)
		if err != nil {
			log.Println("[Chunk] - Error saving chunk: ", err)
			http.Error(w, "Error saving chunk", http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chunk)
	}
}
