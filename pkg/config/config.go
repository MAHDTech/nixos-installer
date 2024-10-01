package config

import (
	"errors"
	"os"

	yaml "gopkg.in/yaml.v3"

	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

type Config struct {
	HostID string `yaml:"hostId" validate:"required"`

	Flake string `yaml:"flake" validate:"required"`

	UEFI struct {
		Label string `yaml:"label" validate:"required"`
		Disk  string `yaml:"disk" validate:"required"`
		Size  string `yaml:"size" validate:"required"`
	} `yaml:"uefi" validate:"required"`

	ZFS struct {
		Pool struct {
			Name        string `yaml:"name" validate:"required"`
			Compression bool   `yaml:"compression" default:"true"`
			Encryption  bool   `yaml:"encryption" default:"false"`
			Mirror      bool   `yaml:"mirror" default:"false"`
			Stripe      bool   `yaml:"stripe" default:"false"`
		} `yaml:"pool" validate:"required"`
		Disks []string `yaml:"disks" validate:"required"`
	} `yaml:"zfs" validate:"required"`

	Swap struct {
		Enabled bool   `yaml:"enabled" default:"false"`
		Size    string `yaml:"size" validate:"required"`
	} `yaml:"swap" validate:"required"`
}

// ReadConfig reads the configuration file.
func ReadConfig(configFile string) (Config, error) {
	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		return Config{}, err
	}

	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}

// ValidateConfig validates the configuration file.
func ValidateConfig(configData *Config) error {
	// Make sure a flake was specified.
	if configData.Flake == "" {
		return errors.New("flake not specified")
	}

	// Check if the UEFI target device is a valid block device.
	if !utils.IsValidBlockDevice(configData.UEFI.Disk) {
		return errors.New("Invalid block device: " + configData.UEFI.Disk)
	}

	// Check if the root disks are valid block devices.
	for _, rootDisk := range configData.ZFS.Disks {
		if !utils.IsValidBlockDevice(rootDisk) {
			return errors.New("Invalid block device: " + rootDisk)
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
			return errors.New("must mirror or stripe, pick one")
		}
	}

	return nil

}
