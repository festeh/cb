package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var serverURL string

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	rootCmd := &cobra.Command{
		Use:   "screen",
		Short: "Screen tool",
	}

	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "http://localhost:8080", "Server URL")

	rootCmd.AddCommand(overlayCmd)
	rootCmd.AddCommand(captureCmd)
	rootCmd.AddCommand(scrollCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
