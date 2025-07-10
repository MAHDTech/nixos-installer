// Package config provides the configuration for the installer.
package config

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	validator "github.com/go-playground/validator/v10"
	yaml "gopkg.in/yaml.v3"

	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// Build-time variables injected by ldflags
var (
	GitHubRepo = "MAHDTech/nixos-installer" // Default fallback
	GitRef     = "trunk"                    // Default fallback
)

// Config is the top-level configuration for the installer.
type Config struct {

	/*
	 NixOS

	 This section defines settings specific to the NixOS installation.
	*/
	NixOS struct {
		// HostID is optional and will be generated if not specified.
		HostID string `yaml:"hostId" default:""`

		// Flake is required.
		Flake string `yaml:"flake" default:"" validate:"required"`

		// The NixOS configuration partition on the uefi disk.
		// This is optional and defaults to disabled.
		Config struct {
			Enabled bool `yaml:"enabled" default:"false"`
		}
	} `yaml:"nixos" validate:"required"`

	/*
	 UEFI

	 This section defines settings for the EFI System Partition (ESP).
	*/
	UEFI struct {
		Label string `yaml:"label" validate:"required"`
		Disk  string `yaml:"disk" validate:"required"`
		Size  string `yaml:"size" validate:"required"`
	} `yaml:"uefi" validate:"required"`

	/*
	 ZFS

	 This section defines settings for the ZFS pool and associated disks.
	*/
	ZFS struct {
		// Configuration settings.
		Ashift int `yaml:"ashift" default:"12"`

		// ZFS Pool configuration.
		Pool struct {
			Name string `yaml:"name" default:"zpool"`

			Compression bool `yaml:"compression" default:"true"`

			// Type can be: single, mirror, stripe, raidz, raidz2, raidz3
			Type string `yaml:"type" default:"single"`

			// Size for the pool (0 means auto-size)
			Size string `yaml:"size" default:"0"`

			// Encryption for the pool
			Encryption bool `yaml:"encryption" default:"false"`

			Disks struct {
				Cache []string `yaml:"cache"`
				Log   []string `yaml:"log"`
				Data  []string `yaml:"data" validate:"required"`
				Spare []string `yaml:"spare"`
			} `yaml:"disks" validate:"required"`
		} `yaml:"pool" validate:"required"`
	} `yaml:"zfs" validate:"required"`

	/*
	 Swap

	 This section defines settings for the swap space (optional).
	*/
	Swap struct {
		Enabled bool   `yaml:"enabled" default:"false"`
		Size    string `yaml:"size" validate:"required"`
	} `yaml:"swap" validate:"required"`
}

// ReadConfig reads and validates the YAML configuration file.
// It supports both local file paths and config names that will be fetched from GitHub.
func ReadConfig(configFile string) (*Config, error) {

	var yamlFile []byte
	var err error

	// Determine if this is a config name or file path
	if isConfigName(configFile) {
		log.Printf("Detected config name '%s', fetching from GitHub...", configFile)
		yamlFile, err = fetchConfigFromGitHub(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch config '%s' from GitHub: %w", configFile, err)
		}
	} else {
		// Original file path logic
		log.Printf("Detected file path '%s', reading local file...", configFile)
		yamlFile, err = readLocalConfigFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read local config file: %w", err)
		}
	}

	// Parse the YAML file.
	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}

	// Apply default values that weren't specified in the YAML
	applyDefaults(&config)

	/*
	 Validation
	*/
	validate := validator.New()

	// 1. Validate basic struct validation using tags (e.g., 'required')
	err = validate.Struct(&config)
	if err != nil {
		// This error can be complex, might need nicer formatting for user
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// 2. Validate custom validation logic
	err = validateConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("custom configuration validation failed: %w", err)
	}

	return &config, nil
}

// applyDefaults sets default values for fields that weren't specified in the YAML
func applyDefaults(config *Config) {

	// Apply defaults for unspecified fields.

	// ZFS ashift
	if config.ZFS.Ashift == 0 {
		config.ZFS.Ashift = 12
		log.Printf("Warning: ZFS ashift value not specified, defaulting to 12 (4K sectors)")
	} else {
		log.Printf("Using ZFS ashift value: %d", config.ZFS.Ashift)
	}

	// ZFS pool type
	if config.ZFS.Pool.Type == "" {
		config.ZFS.Pool.Type = "single"
		log.Printf("Warning: ZFS pool type not specified, defaulting to 'single'")
	}

	// ZFS pool size
	if config.ZFS.Pool.Size == "" {
		config.ZFS.Pool.Size = "0"
		log.Printf("Warning: ZFS pool size not specified, defaulting to auto-size (0)")
	}
}

// validateConfig performs custom validation checks not covered by struct tags.
func validateConfig(configData *Config) error {
	// Validate UEFI configuration
	if err := validateUEFIConfig(configData); err != nil {
		return err
	}

	// Validate ZFS disk configuration
	if err := validateZFSDisks(configData); err != nil {
		return err
	}

	// Validate ZFS pool configuration
	if err := validateZFSPool(configData); err != nil {
		return err
	}

	return nil
}

// validateUEFIConfig validates UEFI-related configuration
func validateUEFIConfig(configData *Config) error {
	// Check if the UEFI target device is a valid block device.
	if !utils.IsValidBlockDevice(configData.UEFI.Disk) {
		return fmt.Errorf("invalid UEFI block device: %s", configData.UEFI.Disk)
	}
	return nil
}

// validateZFSDisks validates that all ZFS disks are valid block devices
func validateZFSDisks(configData *Config) error {
	// Check data disks
	for _, disk := range configData.ZFS.Pool.Disks.Data {
		if !utils.IsValidBlockDevice(disk) {
			return fmt.Errorf("invalid ZFS data disk: %s", disk)
		}
	}

	// Check cache disks
	for _, disk := range configData.ZFS.Pool.Disks.Cache {
		if !utils.IsValidBlockDevice(disk) {
			return fmt.Errorf("invalid ZFS cache disk: %s", disk)
		}
	}

	// Check log disks
	for _, disk := range configData.ZFS.Pool.Disks.Log {
		if !utils.IsValidBlockDevice(disk) {
			return fmt.Errorf("invalid ZFS log disk: %s", disk)
		}
	}

	// Check spare disks
	for _, disk := range configData.ZFS.Pool.Disks.Spare {
		if !utils.IsValidBlockDevice(disk) {
			return fmt.Errorf("invalid ZFS spare disk: %s", disk)
		}
	}

	return nil
}

// validateZFSPool validates ZFS pool configuration including ashift, type, and disk count
func validateZFSPool(configData *Config) error {
	// ZFS ashift validation
	if configData.ZFS.Ashift < 9 || configData.ZFS.Ashift > 16 {
		return fmt.Errorf(
			"invalid ashift value: %d. Must be between 9 and 16",
			configData.ZFS.Ashift,
		)
	}

	// ZFS pool type validation
	if err := validatePoolType(configData.ZFS.Pool.Type); err != nil {
		return err
	}

	// Validate pool configuration based on type and number of data disks
	return validatePoolTypeRequirements(configData.ZFS.Pool.Type, configData.ZFS.Pool.Disks.Data)
}

// validatePoolType validates that the pool type is supported
func validatePoolType(poolType string) error {
	validTypes := []string{"single", "mirror", "stripe", "raidz", "raidz2", "raidz3"}
	for _, validType := range validTypes {
		if poolType == validType {
			return nil
		}
	}
	return fmt.Errorf(
		"invalid ZFS pool type: %s. Must be one of: %v",
		poolType,
		validTypes,
	)
}

// validatePoolTypeRequirements validates disk count requirements for each pool type
func validatePoolTypeRequirements(poolType string, dataDisks []string) error {
	dataDiskCount := len(dataDisks)
	if dataDiskCount == 0 {
		return errors.New("at least one data disk is required for ZFS pool")
	}

	switch poolType {
	case "single":
		if dataDiskCount != 1 {
			return fmt.Errorf(
				"single pool type requires exactly 1 data disk, got %d",
				dataDiskCount,
			)
		}
	case "mirror":
		if dataDiskCount < 2 {
			return fmt.Errorf(
				"mirror pool type requires at least 2 data disks, got %d",
				dataDiskCount,
			)
		}
		if dataDiskCount%2 != 0 {
			return fmt.Errorf(
				"mirror pool type requires an even number of data disks, got %d",
				dataDiskCount,
			)
		}
	case "stripe":
		if dataDiskCount < 1 {
			return fmt.Errorf(
				"stripe pool type requires at least 1 data disk, got %d",
				dataDiskCount,
			)
		}
	case "raidz":
		if dataDiskCount < 3 {
			return fmt.Errorf(
				"raidz pool type requires at least 3 data disks, got %d",
				dataDiskCount,
			)
		}
	case "raidz2":
		if dataDiskCount < 4 {
			return fmt.Errorf(
				"raidz2 pool type requires at least 4 data disks, got %d",
				dataDiskCount,
			)
		}
	case "raidz3":
		if dataDiskCount < 5 {
			return fmt.Errorf(
				"raidz3 pool type requires at least 5 data disks, got %d",
				dataDiskCount,
			)
		}
	}

	return nil
}

// isConfigName determines if the input is a config name (no path separators, no .yaml extension)
// or a file path.
func isConfigName(input string) bool {
	// If it contains path separators or has .yaml extension, treat as file path
	if strings.Contains(input, "/") || strings.Contains(input, "\\") ||
		strings.HasSuffix(input, ".yaml") ||
		strings.HasSuffix(input, ".yml") {
		return false
	}
	// Otherwise, treat as config name
	return true
}

// fetchConfigFromGitHub fetches a config file from the GitHub repository
func fetchConfigFromGitHub(configName string) ([]byte, error) {
	// Construct the GitHub raw URL
	rawURL := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/configs/%s.yaml",
		GitHubRepo,
		GitRef,
		configName,
	)

	log.Printf("Using GitHub repository: %s", GitHubRepo)
	log.Printf("Using Git ref: %s", GitRef)
	log.Printf("Fetching config from: %s", rawURL)

	// Validate the URL to mitigate security issues
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	// Ensure we're only making requests to GitHub
	if parsedURL.Host != "raw.githubusercontent.com" {
		return nil, fmt.Errorf(
			"invalid host: %s, only raw.githubusercontent.com is allowed",
			parsedURL.Host,
		)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make HTTP GET request with validated URL
	resp, err := client.Get(parsedURL.String()) // #nosec G107 - URL is validated above
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Warning: failed to close response body: %v", closeErr)
		}
	}()

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"config '%s' not found on GitHub (HTTP %d)",
			configName,
			resp.StatusCode,
		)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// readLocalConfigFile reads a config file from the local filesystem with security checks
func readLocalConfigFile(configFile string) ([]byte, error) {
	// Get absolute path and clean it
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get absolute path for config file %s: %w",
			configFile,
			err,
		)
	}
	cleanedPath := filepath.Clean(absPath)

	// Security: Ensure the config file path is within the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	if !strings.HasPrefix(cleanedPath, cwd) {
		return nil, fmt.Errorf(
			"config file path %s is outside the current working directory %s",
			cleanedPath,
			cwd,
		)
	}

	// Check if the config file exists (using cleaned path).
	if !utils.FileExists(cleanedPath) {
		return nil, fmt.Errorf("config file not found: %s", cleanedPath)
	}

	// Read the config file.
	// #nosec G304 - Mitigate path traversal.
	yamlFile, err := os.ReadFile(cleanedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", cleanedPath, err)
	}

	return yamlFile, nil
}
