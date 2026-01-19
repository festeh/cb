package main

import (
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var serverURL string

func main() {
	logFile, err := os.OpenFile("/tmp/cb-screen.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	rootCmd := &cobra.Command{
		Use:   "screen",
		Short: "Screen tool",
	}

	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "http://localhost:8080", "Server URL")

	rootCmd.AddCommand(overlayCmd)
	rootCmd.AddCommand(captureCmd)
	rootCmd.AddCommand(scrollCmd)
	rootCmd.AddCommand(cleanCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
