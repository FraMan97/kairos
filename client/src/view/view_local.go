package view

import (
	"net/http"

	"github.com/FraMan97/kairos/client/src/controller"
)

func Start(w http.ResponseWriter, r *http.Request) {
	controller.StartNode(w, r)
}

func Put(w http.ResponseWriter, r *http.Request) {
	controller.PutFile(w, r)
}

func Get(w http.ResponseWriter, r *http.Request) {
	controller.GetFile(w, r)
}
