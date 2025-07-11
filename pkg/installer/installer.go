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
	// Parse command line flags
	configFile, execute, executeInstall, err := parseFlags()
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// Read and validate configuration
	configData, err := config.ReadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("failed to read or validate configuration: %w", err)
	}

	// Ensure the required tools are installed.
	err = checkToolsInstalled()
	if err != nil {
		return fmt.Errorf("required tool is missing: %w", err)
	}

	// Execute installation phases
	if err := runPreparationPhase(*execute, configData); err != nil {
		return err
	}

	partitionInfo, err := runDiskSetupPhase(*execute, configData)
	if err != nil {
		return err
	}

	if err := runZFSSetupPhase(*execute, configData); err != nil {
		return err
	}

	if err := runMountingPhase(*execute, configData, partitionInfo); err != nil {
		return err
	}

	if err := runNixOSConfigurationPhase(*execute); err != nil {
		return err
	}

	if err := runNixOSInstallationPhase(*execute, *executeInstall, configData); err != nil {
		return err
	}

	log.Println("### Finished NixOS installation process ###")
	return nil
}

// parseFlags parses and validates command line flags
func parseFlags() (*string, *bool, *bool, error) {
	configFile := flag.String(
		"config",
		"config.yaml",
		"Path to the YAML configuration file.",
	)
	execute := flag.Bool(
		"run",
		false,
		"Execute mode. (defaults to false which will run in dry-run mode.)",
	)
	executeInstall := flag.Bool(
		"install",
		false,
		"Enable to automatically install NixOS. (defaults to false which only generates the NixOS configuration)",
	)
	flag.Parse()

	if *execute {
		log.Println("Running in execute mode.")
	} else {
		log.Println("Running in dry run mode, see '-help' for more information.")
	}

	return configFile, execute, executeInstall, nil
}

// checkToolsInstalled checks if the required tools are installed
func checkToolsInstalled() error {

	for _, tool := range requiredTools {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("%s", tool)
		}
	}

	return nil

}

// runPreparationPhase handles the preparation phase of installation
func runPreparationPhase(execute bool, configData *config.Config) error {
	log.Println("--- Starting Preparation Phase ---")

	// Check the mountpoints.
	_, err := checkMountpoints(execute)
	if err != nil {
		// Log non-fatal error, checking mounts is informative but not critical for proceeding
		log.Printf("Warning: Failed to check initial mountpoints: %v", err)
	}

	// Create the necessary directories.
	err = createDirectories(execute, mountPoint, configData)
	if err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Umount all partitions on the disks.
	err = unmountDisks(execute, configData)
	if err != nil {
		return fmt.Errorf("failed to unmount disks: %w", err)
	}

	log.Println("--- Preparation Phase Complete ---")
	return nil
}

// runDiskSetupPhase handles disk setup and partitioning
func runDiskSetupPhase(execute bool, configData *config.Config) (PartitionInfo, error) {
	log.Println("--- Starting Disk Setup Phase ---")

	// Wipe and partition the disks.
	partitionInfo, err := wipeAndPartitionDisks(execute, configData)
	if err != nil {
		return PartitionInfo{}, fmt.Errorf("failed during disk partitioning: %w", err)
	}

	log.Println("--- Disk Setup Phase Complete ---")
	return partitionInfo, nil
}

// runZFSSetupPhase handles ZFS pool and dataset creation
func runZFSSetupPhase(execute bool, configData *config.Config) error {
	log.Println("--- Starting ZFS Setup Phase ---")

	// Get disk IDs for ZFS pool
	zfsDiskIDs, err := getZFSDiskIDs(execute, configData)
	if err != nil {
		return err
	}

	// Create the ZFS pool.
	err = createZFSPool(
		execute,
		mountPoint,
		configData.ZFS.Pool.Name,
		configData.ZFS.Pool.Compression,
		configData.ZFS.Pool.Type,
		zfsDiskIDs.Cache,
		zfsDiskIDs.Log,
		zfsDiskIDs.Data,
		zfsDiskIDs.Spare,
		configData.ZFS.Ashift,
	)
	if err != nil {
		return fmt.Errorf("failed to create ZFS pool: %w", err)
	}
	log.Printf("Created ZFS Pool: %s\n", configData.ZFS.Pool.Name)

	// Create the ZFS datasets for the pool.
	err = createZFSDatasets(execute, configData.ZFS.Pool.Name, configData)
	if err != nil {
		return fmt.Errorf(
			"failed to create ZFS datasets on pool %s: %w",
			configData.ZFS.Pool.Name,
			err,
		)
	}

	log.Println("--- ZFS Setup Phase Complete ---")
	return nil
}

// ZFSDiskIDs holds the disk IDs for different ZFS components
type ZFSDiskIDs struct {
	Cache []string
	Log   []string
	Data  []string
	Spare []string
}

// getZFSDiskIDs retrieves and converts disk IDs for ZFS components
func getZFSDiskIDs(execute bool, configData *config.Config) (*ZFSDiskIDs, error) {
	// Users might provide disks in /dev/X format.
	// We need to convert them to /dev/disk/by-id/X format as
	// ZFS pools work better with the by-id format.

	cache, err := getDiskIDsByID(execute, configData.ZFS.Pool.Disks.Cache)
	if err != nil {
		return nil, fmt.Errorf("failed to get ZFS disk IDs for the pool cache: %w", err)
	}

	log, err := getDiskIDsByID(execute, configData.ZFS.Pool.Disks.Log)
	if err != nil {
		return nil, fmt.Errorf("failed to get ZFS disk IDs for the pool log: %w", err)
	}

	data, err := getDiskIDsByID(execute, configData.ZFS.Pool.Disks.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to get ZFS disk IDs for the pool data: %w", err)
	}

	spare, err := getDiskIDsByID(execute, configData.ZFS.Pool.Disks.Spare)
	if err != nil {
		return nil, fmt.Errorf("failed to get ZFS disk IDs for the pool spare: %w", err)
	}

	return &ZFSDiskIDs{
		Cache: cache,
		Log:   log,
		Data:  data,
		Spare: spare,
	}, nil
}

// runMountingPhase handles filesystem mounting
func runMountingPhase(execute bool, configData *config.Config, partitionInfo PartitionInfo) error {
	log.Println("--- Starting Mounting Phase ---")
	err := mountFileSystems(
		execute,
		mountPoint,
		configData,
		partitionInfo,
		configData.ZFS.Pool.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to mount filesystems: %w", err)
	}
	log.Println("--- Mounting Phase Complete ---")
	return nil
}

// runNixOSConfigurationPhase handles NixOS configuration generation
func runNixOSConfigurationPhase(execute bool) error {
	log.Println("--- Starting NixOS Configuration Phase ---")

	// Generate the NixOS configuration.
	err := generateNixOSConfig(execute, mountPoint)
	if err != nil {
		return fmt.Errorf("failed to generate NixOS configuration: %w", err)
	}

	log.Println("--- NixOS Configuration Phase Complete ---")
	return nil
}

// runNixOSInstallationPhase handles the final NixOS installation
func runNixOSInstallationPhase(execute, executeInstall bool, configData *config.Config) error {
	log.Println("--- Starting NixOS Installation Phase ---")

	// Install NixOS.
	err := installNixOS(execute, executeInstall, mountPoint, configData)
	if err != nil {
		return fmt.Errorf("failed during NixOS installation: %w", err)
	}

	log.Println("--- NixOS Installation Phase Complete ---")
	return nil
}
