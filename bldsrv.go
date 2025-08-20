package main

import (
	"fmt"
	"log"
	"net/http"
)

func echoPath(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, r.URL.Path)
}

func main() {
	http.HandleFunc("/", echoPath)
	
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}