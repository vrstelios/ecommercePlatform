package main

import (
	"net/http"
)

func hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("backend-1", "true")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte("Hello from the backend-1 running on port 8081"))
}

/*func main() {
	router := http.NewServeMux()

	router.HandleFunc("/", hello)

	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error staring the server")
	}
}*/
