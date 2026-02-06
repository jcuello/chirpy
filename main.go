package main

import (
	"net/http"
)

func main() {
	serveMux := http.ServeMux{}
	server := http.Server{}

	serveMux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir("."))))
	serveMux.HandleFunc("/healthz", func(resp http.ResponseWriter, request *http.Request) {
		resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resp.WriteHeader(http.StatusOK)
		resp.Write([]byte("OK"))

	})

	server.Handler = &serveMux
	server.Addr = ":8080"

	server.ListenAndServe()

}
