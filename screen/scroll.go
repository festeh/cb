package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var scrollCmd = &cobra.Command{
	Use:   "scroll [up|down]",
	Short: "Scroll the overlay and frontend",
	Args:  cobra.ExactArgs(1),
	Run:   runScroll,
}

func runScroll(cmd *cobra.Command, args []string) {
	dir := args[0]
	if dir != "up" && dir != "down" {
		log.Fatalf("Invalid direction: %s (must be 'up' or 'down')", dir)
	}

	// Send signal to screen overlay (SIGUSR1=up, SIGUSR2=down)
	signal := "-SIGUSR1"
	if dir == "down" {
		signal = "-SIGUSR2"
	}
	if data, err := os.ReadFile(getPidFilePath()); err == nil {
		pid := strings.TrimSpace(string(data))
		exec.Command("kill", signal, pid).Run()
	}

	// Send HTTP request to server for frontend SSE
	resp, err := http.Post(fmt.Sprintf("%s/scroll?dir=%s", serverURL, dir), "", nil)
	if err != nil {
		log.Printf("Failed to notify server: %v", err)
		return
	}
	resp.Body.Close()
}
