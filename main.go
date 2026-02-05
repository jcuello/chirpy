package main

import (
	"net/http"
)

func main() {
	serveMux := http.ServeMux{}
	server := http.Server{}

	serveMux.Handle("/", http.FileServer(http.Dir(".")))

	server.Handler = &serveMux
	server.Addr = ":8080"

	server.ListenAndServe()

}
