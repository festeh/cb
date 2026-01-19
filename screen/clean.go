package main

import (
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Kill overlay and clear flow state",
	Run:   runClean,
}

func runClean(cmd *cobra.Command, args []string) {
	killOverlay()
	log.Printf("Overlay killed")

	resp, err := http.Post(serverURL+"/flow/clear", "", nil)
	if err != nil {
		log.Printf("Failed to clear flow: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("Flow cleared")
}
