package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var imageDir string

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	dir := flag.String("dir", "", "directory to save images (default ~/Pictures/cb)")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("Starting image server")

	if *dir != "" {
		imageDir = *dir
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get home directory: %v", err)
		}
		imageDir = filepath.Join(homeDir, "Pictures", "cb")
	}

	if err := os.MkdirAll(imageDir, 0755); err != nil {
		log.Fatalf("Failed to create image directory: %v", err)
	}

	log.Printf("Image directory: %s", imageDir)

	http.HandleFunc("/upload", handleUpload)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Receiving image from %s", r.RemoteAddr)
	start := time.Now()

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(imageDir, fmt.Sprintf("img_%s.png", timestamp))

	file, err := os.Create(filename)
	if err != nil {
		log.Printf("ERROR: Failed to create file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	written, err := io.Copy(file, r.Body)
	if err != nil {
		log.Printf("ERROR: Failed to write file: %v", err)
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	elapsed := time.Since(start)
	log.Printf("Saved: %s (%d bytes, took %v)", filename, written, elapsed)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Saved: %s", filename)
}
