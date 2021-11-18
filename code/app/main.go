package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var cs CloudStorage

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	fmt.Printf("Port: %s\n", port)

	var err error
	cs, err = NewCloudStorage("scaler-attempt-bucket")
	if err != nil {
		log.Printf("failed to create client: %v", err)
		return
	}
	defer cs.Close()

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/api/v1/image", listHandler).Methods(http.MethodGet, http.MethodOptions)
	// router.HandleFunc("/api/v1/image", createHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/v1/image/{id}", readHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/v1/image/{id}", deleteHandler).Methods(http.MethodDelete)
	// router.HandleFunc("/api/v1/image/{id}", updateHandler).Methods(http.MethodPost, http.MethodPut)

	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})

	log.Fatal(http.ListenAndServe(":"+port, handlers.CORS(originsOk, headersOk, methodsOk)(router)))
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	fs, err := cs.List()
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("failed to list files: %v", err))

		return
	}

	is, err := NewImages(fs)
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("failed to convert files to images images: %v", err))
		return
	}

	writeJSON(w, is, http.StatusOK)
	return
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	fs, err := cs.Read(id)
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("failed to read files %s: %v", id, err))

		return
	}

	is, err := NewImages(fs)
	if err != nil {
		writeErrorMsg(w, fmt.Errorf("failed to convert files to images images: %v", err))
		return
	}
	if len(is) < 1 {
		writeResponse(w, http.StatusNoContent, "")
		return
	}

	writeJSON(w, is[0], http.StatusOK)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	if err := cs.Delete(id); err != nil {
		writeErrorMsg(w, err)
		return
	}
	msg := Message{"image deleted", fmt.Sprintf("image id: %s", id)}

	writeJSON(w, msg, http.StatusNoContent)
}

// JSONProducer is an interface that spits out a JSON string version of itself
type JSONProducer interface {
	JSON() (string, error)
	JSONBytes() ([]byte, error)
}

func writeJSON(w http.ResponseWriter, j JSONProducer, status int) {
	json, err := j.JSON()
	if err != nil {
		writeErrorMsg(w, err)
		return
	}
	writeResponse(w, status, json)
	return
}

func writeErrorMsg(w http.ResponseWriter, err error) {
	s := fmt.Sprintf("{\"error\":\"%s\"}", err)
	writeResponse(w, http.StatusInternalServerError, s)
	return
}

func writeResponse(w http.ResponseWriter, status int, msg string) {
	if status != http.StatusOK {
		weblog(fmt.Sprintf(msg))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,access-control-allow-origin, access-control-allow-headers")
	w.WriteHeader(status)
	w.Write([]byte(msg))

	return
}

func weblog(msg string) {
	log.Printf("Webserver : %s", msg)
}

// Message is a structure for communicating additional data to API consumer.
type Message struct {
	Text    string `json:"text"`
	Details string `json:"details"`
}

// JSON marshalls the content of a todo to json.
func (m Message) JSON() (string, error) {
	bytes, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("could not marshal json for response: %s", err)
	}

	return string(bytes), nil
}

// JSONBytes marshalls the content of a todo to json as a byte array.
func (m Message) JSONBytes() ([]byte, error) {
	bytes, err := json.Marshal(m)
	if err != nil {
		return []byte{}, fmt.Errorf("could not marshal json for response: %s", err)
	}

	return bytes, nil
}
