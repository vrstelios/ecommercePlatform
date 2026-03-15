package main

/*func hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("backend-3", "true")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte("Hello from the backend-1 running on port 8083"))
}

func main() {
	router := http.NewServeMux()

	router.HandleFunc("/", hello)

	server := &http.Server{
		Addr:    ":8083",
		Handler: router,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error staring the server")
	}
}*/
