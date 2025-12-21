package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"
)

var (
	serverURL   string
	monitorName string
)

func main() {
	server := flag.String("server", "localhost:8080", "server address (host:port)")
	interval := flag.Int("interval", 10, "screenshot interval in seconds")
	monitor := flag.String("monitor", "", "monitor/output name (e.g., DP-1, HDMI-A-1)")
	oneshot := flag.Bool("oneshot", false, "take single screenshot and exit")
	flag.Parse()

	serverURL = fmt.Sprintf("http://%s/upload", *server)
	monitorName = *monitor

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	if monitorName != "" {
		if err := validateMonitor(monitorName); err != nil {
			log.Fatalf("Invalid monitor: %v", err)
		}
	}

	// One-shot mode: take screenshot and exit
	if *oneshot {
		takeAndSendScreenshot()
		return
	}

	log.Println("Starting screenshot client")
	log.Printf("Server URL: %s", serverURL)
	log.Printf("Interval: %d seconds", *interval)
	if monitorName != "" {
		log.Printf("Monitor: %s", monitorName)
	}

	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()

	// Take first screenshot immediately
	takeAndSendScreenshot()

	for range ticker.C {
		takeAndSendScreenshot()
	}
}

func validateMonitor(name string) error {
	// Try hyprctl first, then swaymsg
	monitors, err := listMonitors()
	if err != nil {
		return err
	}

	for _, m := range monitors {
		if m == name {
			return nil
		}
	}

	return fmt.Errorf("monitor '%s' not found. Available: %v", name, monitors)
}

func listMonitors() ([]string, error) {
	// Try hyprctl monitors
	cmd := exec.Command("hyprctl", "monitors", "-j")
	output, err := cmd.Output()
	if err == nil {
		return parseHyprctlMonitors(output)
	}

	// Try swaymsg
	cmd = exec.Command("swaymsg", "-t", "get_outputs")
	output, err = cmd.Output()
	if err == nil {
		return parseSwayOutputs(output)
	}

	return nil, fmt.Errorf("could not list monitors (tried hyprctl and swaymsg)")
}

func parseHyprctlMonitors(data []byte) ([]string, error) {
	var monitors []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &monitors); err != nil {
		return nil, err
	}
	names := make([]string, len(monitors))
	for i, m := range monitors {
		names[i] = m.Name
	}
	return names, nil
}

func parseSwayOutputs(data []byte) ([]string, error) {
	var outputs []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &outputs); err != nil {
		return nil, err
	}
	names := make([]string, len(outputs))
	for i, o := range outputs {
		names[i] = o.Name
	}
	return names, nil
}

func takeAndSendScreenshot() {
	log.Printf("Taking screenshot...")
	start := time.Now()

	// Capture screenshot to stdout
	var cmd *exec.Cmd
	if monitorName != "" {
		cmd = exec.Command("grim", "-o", monitorName, "-")
	} else {
		cmd = exec.Command("grim", "-")
	}
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
	log.Printf("Uploaded (%d bytes, send took %v, total %v)", len(output), sendTime, totalTime)
}
