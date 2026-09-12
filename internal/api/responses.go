package api

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

type provisionResponse struct {
	ServerName    string         `json:"server_name"`
	Status        string         `json:"status"`
	ServerOptions k8s.ServerOpts `json:"server_options"`
}

type DeleteServerResponse struct {
	ServerName    string `json:"server_name"`
	Status        string `json:"status"`
	StoragePurged bool   `json:"storage_purged"`
}

type messageJson struct {
	Message string `json:"message"`
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal json for json response: %v", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func wantsHTML(r *http.Request) bool {
	h := r.Header.Get("Accept")
	if strings.Contains(h, "text/html") {
		return true
	}
	return false
}

func respondWithHTML(w http.ResponseWriter, code int, t *template.Template, name string, data interface{}) {
	buffer := &bytes.Buffer{}

	if err := t.ExecuteTemplate(buffer, name, data); err != nil {
		log.Printf("failed to execute template '%s': %v", name, err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	w.Write(buffer.Bytes())
}
