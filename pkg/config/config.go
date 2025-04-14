// Package config provides the configuration for the installer.
package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	validator "github.com/go-playground/validator/v10"
	yaml "gopkg.in/yaml.v3"

	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
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

		// ZFS Boot Pool configuration.
		BootPool struct {
			Name        string `yaml:"name" default:"bpool"`
			Compression bool   `yaml:"compression" default:"true"`
			Size        string `yaml:"size" validate:"required" default:"5G"`
			Mirror      bool   `yaml:"mirror" default:"false"`
			Stripe      bool   `yaml:"stripe" default:"false"`
		} `yaml:"boot" validate:"required"`

		// ZFS Root Pool configuration.
		RootPool struct {
			Name        string `yaml:"name" default:"zpool"`
			Compression bool   `yaml:"compression" default:"true"`
			Encryption  bool   `yaml:"encryption" default:"false"`
			Mirror      bool   `yaml:"mirror" default:"false"`
			Stripe      bool   `yaml:"stripe" default:"false"`
		} `yaml:"root" validate:"required"`

		// ZFS Disks configuration.
		Disks []string `yaml:"disks" validate:"required"`
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
	}
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

	// ZFS ashift validation.
	if configData.ZFS.Ashift < 9 || configData.ZFS.Ashift > 16 {
		return fmt.Errorf(
			"invalid ashift value: %d. Must be between 9 and 16",
			configData.ZFS.Ashift,
		)
	}

	// If there is more than disk, are we mirroring or striping the boot and root pools?
	if len(configData.ZFS.Disks) > 1 {

		// Boot pool
		if configData.ZFS.BootPool.Mirror && configData.ZFS.BootPool.Stripe {
			return errors.New("can't mirror and stripe the boot pool, pick one")
		}
		if !configData.ZFS.BootPool.Mirror && !configData.ZFS.BootPool.Stripe {
			return errors.New("must mirror or stripe the boot pool with multiple disks, pick one")
		}

		// Root pool
		if configData.ZFS.RootPool.Mirror && configData.ZFS.RootPool.Stripe {
			return errors.New("can't mirror and stripe the root pool, pick one")
		}
		if !configData.ZFS.RootPool.Mirror && !configData.ZFS.RootPool.Stripe {
			return errors.New("must mirror or stripe the root pool with multiple disks, pick one")
		}

	}

	return nil
}
