package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	imageDir    string
	frontendDir string

	// SSE clients
	sseClients   = make(map[chan string]bool)
	sseClientsMu sync.Mutex
)

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	dir := flag.String("dir", "", "directory to save images (default ~/Pictures/cb)")
	frontend := flag.String("frontend", "", "frontend directory (default ./frontend)")
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

	if *frontend != "" {
		frontendDir = *frontend
	} else {
		frontendDir = "frontend"
	}

	if err := os.MkdirAll(imageDir, 0755); err != nil {
		log.Fatalf("Failed to create image directory: %v", err)
	}

	log.Printf("Image directory: %s", imageDir)
	log.Printf("Frontend directory: %s", frontendDir)

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/images", handleImages)
	http.HandleFunc("/images/", handleImageFile)
	http.HandleFunc("/events", handleSSE)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Receiving image from %s", r.RemoteAddr)
	start := time.Now()

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("img_%s.png", timestamp)
	fullPath := filepath.Join(imageDir, filename)

	file, err := os.Create(fullPath)
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
	log.Printf("Saved: %s (%d bytes, took %v)", fullPath, written, elapsed)

	// Notify SSE clients
	broadcastSSE(filename)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Saved: %s", fullPath)
}

func handleImages(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		log.Printf("ERROR: Failed to read image directory: %v", err)
		http.Error(w, "Failed to list images", http.StatusInternalServerError)
		return
	}

	var images []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			images = append(images, entry.Name())
		}
	}

	// Sort by name descending (newest first due to timestamp naming)
	sort.Sort(sort.Reverse(sort.StringSlice(images)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

func handleImageFile(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/images/")

	// Prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(imageDir, filename)
	http.ServeFile(w, r, fullPath)
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientChan := make(chan string, 10)

	sseClientsMu.Lock()
	sseClients[clientChan] = true
	sseClientsMu.Unlock()

	log.Printf("SSE client connected: %s (total: %d)", r.RemoteAddr, len(sseClients))

	defer func() {
		sseClientsMu.Lock()
		delete(sseClients, clientChan)
		sseClientsMu.Unlock()
		close(clientChan)
		log.Printf("SSE client disconnected: %s (total: %d)", r.RemoteAddr, len(sseClients))
	}()

	// Heartbeat ticker
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case msg := <-clientChan:
			fmt.Fprintf(w, "event: newimage\ndata: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

func broadcastSSE(filename string) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()

	for clientChan := range sseClients {
		select {
		case clientChan <- filename:
		default:
			// Client buffer full, skip
		}
	}
}
