// Package config provides the configuration for the installer.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	yaml "gopkg.in/yaml.v3"

	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// Config is the top-level configuration for the installer.
type Config struct {

	// NixOS defines settings specific to the NixOS installation.
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

	// UEFI defines settings for the EFI System Partition (ESP).
	UEFI struct {
		Label string `yaml:"label" validate:"required"`
		Disk  string `yaml:"disk" validate:"required"`
		Size  string `yaml:"size" validate:"required"`
	} `yaml:"uefi" validate:"required"`

	// ZFS defines settings for the ZFS pool and associated disks.
	ZFS struct {
		Pool struct {
			Name        string `yaml:"name" default:"zpool"`
			Compression bool   `yaml:"compression" default:"true"`
			Encryption  bool   `yaml:"encryption" default:"false"`
			Mirror      bool   `yaml:"mirror" default:"false"`
			Stripe      bool   `yaml:"stripe" default:"false"`
		} `yaml:"pool" validate:"required"`
		Disks []string `yaml:"disks" validate:"required"`
	} `yaml:"zfs" validate:"required"`

	// Swap defines settings for the swap space (optional).
	Swap struct {
		Enabled bool   `yaml:"enabled" default:"false"`
		Size    string `yaml:"size" validate:"required"`
	} `yaml:"swap" validate:"required"`
}

// ReadConfig reads and validates the configuration file.
func ReadConfig(configFile string) (*Config, error) {
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

	// Parse the YAML file.
	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", cleanedPath, err)
	}

	// --- Validation ---
	validate := validator.New()

	// 1. Basic struct validation using tags (e.g., 'required')
	err = validate.Struct(&config)
	if err != nil {
		// This error can be complex, might need nicer formatting for user
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// 2. Custom validation logic
	err = validateConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("custom configuration validation failed: %w", err)
	}

	return &config, nil
}

// validateConfig performs custom validation checks not covered by struct tags.
func validateConfig(configData *Config) error {
	// Check if the UEFI target device is a valid block device.
	if !utils.IsValidBlockDevice(configData.UEFI.Disk) {
		return fmt.Errorf("invalid UEFI block device: %s", configData.UEFI.Disk)
	}

	// Check if the root disks are valid block devices.
	for _, rootDisk := range configData.ZFS.Disks {
		if !utils.IsValidBlockDevice(rootDisk) {
			return fmt.Errorf("invalid ZFS block device: %s", rootDisk)
		}
	}

	// If there is more than one root disk, are we mirroring or striping?
	if len(configData.ZFS.Disks) > 1 {
		// We can't do both.
		if configData.ZFS.Pool.Mirror && configData.ZFS.Pool.Stripe {
			return errors.New("can't mirror and stripe, pick one")
		}
		// But we must do one.
		if !configData.ZFS.Pool.Mirror && !configData.ZFS.Pool.Stripe {
			return errors.New("must mirror or stripe with multiple disks, pick one")
		}
	}

	return nil
}
