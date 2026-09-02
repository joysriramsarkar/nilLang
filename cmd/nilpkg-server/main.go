package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/registry"
)

func main() {
	// Parse command line flags
	host := flag.String("host", "0.0.0.0", "Server host address")
	port := flag.Int("port", 8080, "Server port")
	dataDir := flag.String("data", "./registry-data", "Data directory")
	maxUpload := flag.Int("max-upload", 100, "Max upload size in MB")
	version := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *version {
		fmt.Println("Nilang Registry Server v0.1.0")
		fmt.Println("Alap Framework • Onuron OS")
		os.Exit(0)
	}

	config := &registry.ServerConfig{
		Host:        *host,
		Port:        *port,
		DataDir:     *dataDir,
		MaxUploadMB: *maxUpload,
		EnableAuth:  false,
		LogLevel:    "info",
	}

	server, err := registry.NewServer(config)
	if err != nil {
		log.Fatalf("❌ সার্ভার তৈরি করতে সমস্যা: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("❌ সার্ভার চালাতে সমস্যা: %v", err)
	}
}