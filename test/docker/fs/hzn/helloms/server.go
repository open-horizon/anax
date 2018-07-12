package main

import (
	"io"
	"net/http"
)

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello world!")
}

func movie(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Star Wars: Rouge 1")
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", hello)
	mux.HandleFunc("/movie", movie)
	http.ListenAndServe(":8000", mux)
}
