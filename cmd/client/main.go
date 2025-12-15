package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"
)

var serverURL string

func main() {
	server := flag.String("server", "localhost:8080", "server address (host:port)")
	interval := flag.Int("interval", 10, "screenshot interval in seconds")
	flag.Parse()

	serverURL = fmt.Sprintf("http://%s/upload", *server)

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("Starting screenshot client")
	log.Printf("Server URL: %s", serverURL)
	log.Printf("Interval: %d seconds", *interval)

	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()

	// Take first screenshot immediately
	takeAndSendScreenshot()

	for range ticker.C {
		takeAndSendScreenshot()
	}
}

func takeAndSendScreenshot() {
	log.Printf("Taking screenshot...")
	start := time.Now()

	// Capture screenshot to stdout
	cmd := exec.Command("grim", "-")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("ERROR: Failed to take screenshot: %v", err)
		return
	}

	captureTime := time.Since(start)
	log.Printf("Screenshot captured (%d bytes, took %v)", len(output), captureTime)

	// Send to server
	log.Printf("Sending to server...")
	sendStart := time.Now()

	resp, err := http.Post(serverURL, "image/png", bytes.NewReader(output))
	if err != nil {
		log.Printf("ERROR: Failed to send screenshot: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	sendTime := time.Since(sendStart)

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: Server returned %d: %s", resp.StatusCode, string(body))
		return
	}

	totalTime := time.Since(start)
	log.Printf("Success: %s (send took %v, total %v)", string(body), sendTime, totalTime)
}
