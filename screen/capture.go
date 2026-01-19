package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var (
	captureInterval int
	captureMonitor  string
	captureOneshot  bool
	captureOverlay  bool
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture screenshots and send to server",
	Run:   runCapture,
}

func init() {
	captureCmd.Flags().IntVarP(&captureInterval, "interval", "i", 10, "Screenshot interval in seconds")
	captureCmd.Flags().StringVarP(&captureMonitor, "monitor", "m", "", "Monitor/output name (e.g., DP-1, HDMI-A-1)")
	captureCmd.Flags().BoolVarP(&captureOneshot, "oneshot", "o", false, "Take single screenshot and exit")
	captureCmd.Flags().BoolVar(&captureOverlay, "overlay", false, "Open overlay after oneshot capture")
}

func runCapture(cmd *cobra.Command, args []string) {
	if captureMonitor != "" {
		if err := validateMonitor(captureMonitor); err != nil {
			log.Fatalf("Invalid monitor: %v", err)
		}
	}

	if captureOneshot {
		takeAndSendScreenshot()
		if captureOverlay {
			// Trigger flow before opening overlay
			http.Post(serverURL+"/flow/trigger", "", nil)
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "overlay", "-s", serverURL)
			cmd.Start()
		}
		return
	}

	log.Println("Starting screenshot capture")
	log.Printf("Server URL: %s", serverURL)
	log.Printf("Interval: %d seconds", captureInterval)
	if captureMonitor != "" {
		log.Printf("Monitor: %s", captureMonitor)
	}

	ticker := time.NewTicker(time.Duration(captureInterval) * time.Second)
	defer ticker.Stop()

	takeAndSendScreenshot()

	for range ticker.C {
		takeAndSendScreenshot()
	}
}

func validateMonitor(name string) error {
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
	cmd := exec.Command("hyprctl", "monitors", "-j")
	output, err := cmd.Output()
	if err == nil {
		return parseHyprctlMonitors(output)
	}

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

	var cmd *exec.Cmd
	if captureMonitor != "" {
		cmd = exec.Command("grim", "-o", captureMonitor, "-")
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

	log.Printf("Sending to server...")
	sendStart := time.Now()

	uploadURL := serverURL + "/upload"
	resp, err := http.Post(uploadURL, "image/png", bytes.NewReader(output))
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
