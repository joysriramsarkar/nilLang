package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joysriramsarkar/nilLang/pkg/registry"
)

// authEnvVar can enable or disable required authentication when the -auth
// flag is not given explicitly (e.g. in a container environment).
const authEnvVar = "NILANG_REGISTRY_AUTH"

func main() {
	// Parse command line flags
	host := flag.String("host", "0.0.0.0", "Server host address")
	port := flag.Int("port", 8080, "Server port")
	dataDir := flag.String("data", "./registry-data", "Data directory")
	maxUpload := flag.Int("max-upload", 100, "Max upload size in MB")
	// Auth defaults to ON: with auth off and users registered, a valid key
	// is still required for publish/delete, but a fresh (or user-wiped)
	// data directory silently allows anonymous bootstrap publishes, and
	// ownership checks have no identity to bind to. It stays configurable
	// for local development, where anonymous bootstrap is convenient.
	enableAuth := flag.Bool("auth", true, "Require API-key authentication (env "+authEnvVar+" when flag not set)")
	version := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *version {
		fmt.Println("Nilang Registry Server v0.1.0")
		fmt.Println("Alap Framework • Onuron OS")
		os.Exit(0)
	}

	auth := resolveAuth(*enableAuth)

	config := &registry.ServerConfig{
		Host:        *host,
		Port:        *port,
		DataDir:     *dataDir,
		MaxUploadMB: *maxUpload,
		EnableAuth:  auth,
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

// resolveAuth applies the environment override when -auth was not set
// explicitly on the command line. An unparsable env value is ignored
// (the flag value stands) rather than guessed at.
func resolveAuth(flagValue bool) bool {
	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "auth" {
			explicit = true
		}
	})
	if explicit {
		return flagValue
	}
	if v := os.Getenv(authEnvVar); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
		log.Printf("⚠️  ignoring unparsable %s=%q", authEnvVar, v)
	}
	return flagValue
}
