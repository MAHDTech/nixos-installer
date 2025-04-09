// Package installer contains the logic for installing NixOS.
package installer

import (
	"flag"
	"fmt"
	"log"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
)

// Run function orchestrates the NixOS installation process.
// Returns an error if any step of the installation fails.
func Run() error {

	// --- Configuration and Flags ---
	configFile := flag.String(
		"config",
		"config.yaml",
		"Path to the YAML configuration file.",
	)
	execute := flag.Bool(
		"run",
		false,
		"Execute mode. (default is false which only dry runs commands)",
	)
	executeInstall := flag.Bool(
		"install",
		false,
		"Automatically install NixOS. (default is false which only generates the NixOS configuration)",
	)
	flag.Parse()

	if *execute {
		log.Println("Running in execute mode.")
	} else {
		log.Println("Running in dry run mode, see '-help' for more information.")
	}

	// Read and validate configuration
	configData, err := config.ReadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("failed to read or validate configuration: %w", err)
	}

	// --- Preparation Phase ---
	log.Println("--- Starting Preparation Phase ---")
	_, err = checkMountpoints(*execute) // We don't use the output here
	if err != nil {
		// Log non-fatal error, checking mounts is informative but not critical for proceeding
		log.Printf("Warning: Failed to check initial mountpoints: %v", err)
	}
	err = createDirectories(*execute, "/mnt", configData)
	if err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	log.Println("--- Preparation Phase Complete ---")

	// --- Disk Setup Phase ---
	log.Println("--- Starting Disk Setup Phase ---")
	partitionNameUEFI, partitionNameNixOSConfig, _, _, err := partitionDisks(*execute, configData)
	if err != nil {
		return fmt.Errorf("failed during disk partitioning: %w", err)
	}
	zfsDiskIDs, err := getZFSDiskIDs(configData.ZFS.Disks)
	if err != nil {
		return fmt.Errorf("failed to get ZFS disk IDs: %w", err)
	}
	log.Println("--- Disk Setup Phase Complete ---")

	// --- ZFS Setup Phase ---
	log.Println("--- Starting ZFS Setup Phase ---")
	_, zfsPoolRootName, err := createZFSPool(*execute, "/mnt", configData, zfsDiskIDs)
	if err != nil {
		return fmt.Errorf("failed to create ZFS pool: %w", err)
	}
	err = createZFSDatasets(*execute, zfsPoolRootName, "/mnt", configData)
	if err != nil {
		return fmt.Errorf("failed to create ZFS datasets: %w", err)
	}
	log.Println("--- ZFS Setup Phase Complete ---")

	// --- Mounting Phase ---
	log.Println("--- Starting Mounting Phase ---")
	err = mountFileSystems(
		*execute,
		mountPoint,
		configData,
		partitionNameUEFI,
		partitionNameNixOSConfig,
		zfsPoolRootName,
	)
	if err != nil {
		return fmt.Errorf("failed to mount filesystems: %w", err)
	}
	log.Println("--- Mounting Phase Complete ---")

	// --- NixOS Installation Phase ---
	log.Println("--- Starting NixOS Installation Phase ---")
	err = generateNixOSConfig(*execute, "/mnt")
	if err != nil {
		return fmt.Errorf("failed to generate NixOS configuration: %w", err)
	}
	err = modifyNixOSConfig(*execute, "/mnt", configData)
	if err != nil {
		return fmt.Errorf("failed to modify NixOS configuration: %w", err)
	}
	err = installNixOS(*execute, *executeInstall, "/mnt", configData)
	if err != nil {
		return fmt.Errorf("failed during NixOS installation: %w", err)
	}
	log.Println("--- NixOS Installation Phase Complete ---")

	log.Println("NixOS installation process finished.")
	return nil // Indicate success
}
