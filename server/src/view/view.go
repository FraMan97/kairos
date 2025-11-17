package view

import (
	"net/http"

	"github.com/FraMan97/kairos/server/src/controller"
)

func Subscribe(w http.ResponseWriter, r *http.Request) {
	controller.SubsribeNode(w, r)
}

func Synchronize(w http.ResponseWriter, r *http.Request) {
	controller.SynchronizeData(w, r)
}

func NodesForFileUpload(w http.ResponseWriter, r *http.Request) {
	controller.RequestNodesForFileUpload(w, r)
}

func InsertManifest(w http.ResponseWriter, r *http.Request) {
	controller.InsertFileManifest(w, r)
}

func DownloadManifest(w http.ResponseWriter, r *http.Request) {
	controller.DownloadFileManifest(w, r)
}
