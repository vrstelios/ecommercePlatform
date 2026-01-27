package main

import (
	"log"
	"net/http"
)

func hello(w http.ResponseWriter, r *http.Request) {
	log.Print("Hello product page")
	w.Header().Add("backend-2", "true")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte("Hello from the backend-2"))
}

func main() {
	router := http.NewServeMux()

	router.HandleFunc("/", hello)

	server := &http.Server{
		Addr:    ":8082",
		Handler: router,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error string the server")
	}
}
