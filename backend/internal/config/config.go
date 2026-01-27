package config

import (
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	// HaulerDir is the base directory for hauler data (default: /data)
	HaulerDir string

	// HaulerStoreDir is where hauler stores content (default: /data/store)
	HaulerStoreDir string

	// HaulerTempDir is for temporary files (default: /data/tmp)
	HaulerTempDir string

	// DockerAuthPath is where registry credentials are stored (default: /data/.docker/config.json)
	DockerAuthPath string

	// DatabasePath is the SQLite database file path (default: /data/app.db)
	DatabasePath string

	// DataDir is the base data directory (same as HaulerDir for downloads)
	DataDir string

	// UIPassword is the optional password for UI access (default: empty, no auth)
	UIPassword string
}

// Load returns the application configuration from environment variables
// or defaults if not set.
func Load() *Config {
	haulerDir := getEnv("HAULER_DIR", "/data")
	homeDir := getEnv("HOME", haulerDir)
	dockerConfig := getEnv("DOCKER_CONFIG", filepath.Join(homeDir, ".docker"))

	return &Config{
		HaulerDir:      haulerDir,
		HaulerStoreDir: getEnv("HAULER_STORE_DIR", filepath.Join(haulerDir, "store")),
		HaulerTempDir:  getEnv("HAULER_TEMP_DIR", filepath.Join(haulerDir, "tmp")),
		DockerAuthPath: filepath.Join(dockerConfig, "config.json"),
		DatabasePath:   getEnv("DATABASE_PATH", filepath.Join(haulerDir, "app.db")),
		DataDir:        haulerDir,
		UIPassword:     getEnv("HAULER_UI_PASSWORD", ""),
	}
}

// getEnv returns the environment variable value or the fallback if not set
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// ToMap returns a map representation of the config for JSON serialization
func (c *Config) ToMap() map[string]string {
	return map[string]string{
		"haulerDir":       c.HaulerDir,
		"haulerStoreDir":  c.HaulerStoreDir,
		"haulerTempDir":   c.HaulerTempDir,
		"dockerAuthPath":  c.DockerAuthPath,
		"databasePath":    c.DatabasePath,
		"haulerDirEnv":    "HAULER_DIR",
		"haulerStoreEnv":  "HAULER_STORE_DIR",
		"haulerTempEnv":   "HAULER_TEMP_DIR",
		"dockerConfigEnv": "DOCKER_CONFIG",
		"databasePathEnv": "DATABASE_PATH",
		"authEnabled":     boolToString(c.UIPassword != ""),
	}
}

// boolToString converts a bool to "true" or "false" string
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
