// Package installer contains the logic for installing NixOS.
package installer

import (
	"fmt"
	"os/exec"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	sysutil "github.com/MAHDTech/nixos-installer/pkg/sysutil"
)

// Run function orchestrates the NixOS installation process.
// Returns an error if any step of the installation fails.
func Run(configFile string, execute bool, executeInstall bool) error {
	sysutil.Section("NixOS Installation Process")

	// Read and validate configuration
	sysutil.Info("Reading and validating configuration from %s", configFile)
	configData, err := config.ReadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to read or validate configuration: %w", err)
	}
	sysutil.Success("Configuration loaded successfully")

	// Ensure the required tools are installed.
	err = checkToolsInstalled()
	if err != nil {
		return fmt.Errorf("required tool is missing: %w", err)
	}

	// Create progress tracker for installation phases
	progress := sysutil.NewProgress("Installation Progress", 6)
	progress.Update(0)

	// Execute installation phases
	sysutil.Info("Starting installation phases...")

	progress.Increment()
	if err := runPreparationPhase(execute, configData); err != nil {
		return err
	}

	progress.Increment()
	partitionInfo, err := runDiskSetupPhase(execute, configData)
	if err != nil {
		return err
	}

	progress.Increment()
	if err := runZFSSetupPhase(execute, configData); err != nil {
		return err
	}

	progress.Increment()
	if err := runMountingPhase(execute, configData, partitionInfo); err != nil {
		return err
	}

	progress.Increment()
	if err := runNixOSConfigurationPhase(execute); err != nil {
		return err
	}

	progress.Increment()
	if err := runNixOSInstallationPhase(execute, executeInstall, configData); err != nil {
		return err
	}

	progress.Complete()
	sysutil.Success("NixOS installation process completed successfully")
	return nil
}

// checkToolsInstalled checks if the required tools are installed
func checkToolsInstalled() error {
	sysutil.SubSection("Tool Availability Check")

	missingTools := []string{}
	toolProgress := sysutil.NewProgress("Checking Tools", len(requiredTools))

	for i, tool := range requiredTools {
		if _, err := exec.LookPath(tool); err != nil {
			missingTools = append(missingTools, tool)
			sysutil.Warn("Missing tool: %s", tool)
		} else {
			sysutil.Debug("Found tool: %s", tool)
		}
		toolProgress.Update(i + 1)
	}

	toolProgress.Complete()

	if len(missingTools) > 0 {
		return fmt.Errorf("missing required tools: %v", missingTools)
	}

	sysutil.Success("All required tools are available")
	return nil
}

// runPreparationPhase handles the preparation phase of installation
func runPreparationPhase(execute bool, configData *config.Config) error {
	sysutil.SubSection("Preparation Phase")

	// Check the mountpoints.
	sysutil.Info("Checking current mountpoints")
	_, err := checkMountpoints(execute)
	if err != nil {
		// Log non-fatal error, checking mounts is informative but not critical for proceeding
		sysutil.Warn("Failed to check initial mountpoints: %v", err)
	}

	// Create the necessary directories.
	sysutil.Info("Creating necessary directories")
	err = createDirectories(execute, mountPoint, configData)
	if err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	sysutil.Success("Directories created successfully")

	// Umount all partitions on the disks.
	sysutil.Info("Unmounting existing partitions")
	err = unmountDisks(execute, configData)
	if err != nil {
		return fmt.Errorf("failed to unmount disks: %w", err)
	}
	sysutil.Success("Partitions unmounted successfully")

	sysutil.Success("Preparation phase completed")
	return nil
}

// runDiskSetupPhase handles disk setup and partitioning
func runDiskSetupPhase(execute bool, configData *config.Config) (PartitionInfo, error) {
	sysutil.SubSection("Disk Setup Phase")

	// Wipe and partition the disks.
	sysutil.Info("Wiping and partitioning disks")
	partitionInfo, err := wipeAndPartitionDisks(execute, configData)
	if err != nil {
		return PartitionInfo{}, fmt.Errorf("failed during disk partitioning: %w", err)
	}

	sysutil.Success("Disk setup phase completed")
	return partitionInfo, nil
}

// runZFSSetupPhase handles ZFS pool and dataset creation
func runZFSSetupPhase(execute bool, configData *config.Config) error {
	sysutil.SubSection("ZFS Setup Phase")

	// Get disk IDs for ZFS pool
	sysutil.Info("Retrieving disk IDs for ZFS pool")
	zfsDiskIDs, err := getZFSDiskIDs(execute, configData)
	if err != nil {
		return err
	}

	// Create the ZFS pool.
	sysutil.Info("Creating ZFS pool: %s", configData.ZFS.Pool.Name)
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
	sysutil.Success("Created ZFS Pool: %s", configData.ZFS.Pool.Name)

	// Create the ZFS datasets for the pool.
	sysutil.Info("Creating ZFS datasets")
	err = createZFSDatasets(execute, configData.ZFS.Pool.Name, configData)
	if err != nil {
		return fmt.Errorf(
			"failed to create ZFS datasets on pool %s: %w",
			configData.ZFS.Pool.Name,
			err,
		)
	}

	sysutil.Success("ZFS setup phase completed")
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
	sysutil.SubSection("Mounting Phase")
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
	sysutil.Success("Mounting phase completed")
	return nil
}

// runNixOSConfigurationPhase handles NixOS configuration generation
func runNixOSConfigurationPhase(execute bool) error {
	sysutil.SubSection("NixOS Configuration Phase")

	// Generate the NixOS configuration.
	err := generateNixOSConfig(execute, mountPoint)
	if err != nil {
		return fmt.Errorf("failed to generate NixOS configuration: %w", err)
	}

	sysutil.Success("NixOS configuration phase completed")
	return nil
}

// runNixOSInstallationPhase handles the final NixOS installation
func runNixOSInstallationPhase(execute, executeInstall bool, configData *config.Config) error {
	sysutil.SubSection("NixOS Installation Phase")

	// Install NixOS.
	err := installNixOS(execute, executeInstall, mountPoint, configData)
	if err != nil {
		return fmt.Errorf("failed during NixOS installation: %w", err)
	}

	sysutil.Success("NixOS installation phase completed")
	return nil
}
