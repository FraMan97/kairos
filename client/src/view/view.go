package view

import (
	"net/http"

	"github.com/FraMan97/kairos/client/src/controller"
)

func Chunk(w http.ResponseWriter, r *http.Request) {
	controller.Chunk(w, r)
}
