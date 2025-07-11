package installer

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	sysutil "github.com/MAHDTech/nixos-installer/pkg/sysutil"
)

// createZFSPool creates the ZFS pool.
func createZFSPool(
	execute bool,
	mountPoint string,
	zfsPoolName string,
	zfsPoolCompression bool,
	zfsPoolType string,
	zfsDiskIDsCache []string,
	zfsDiskIDsLog []string,
	zfsDiskIDsData []string,
	zfsDiskIDsSpare []string,
	ashift int,
) error {

	// Prepare all partition IDs
	partitions := prepareZFSPartitions(
		zfsDiskIDsCache,
		zfsDiskIDsLog,
		zfsDiskIDsData,
		zfsDiskIDsSpare,
	)

	// Build pool configuration
	zfsPoolArgs, err := buildZFSPoolArgs(zfsPoolType, partitions)
	if err != nil {
		return err
	}

	// Create the pool
	return executeZFSPoolCreation(
		execute,
		mountPoint,
		zfsPoolName,
		zfsPoolCompression,
		ashift,
		zfsPoolArgs,
		partitions,
	)
}

// ZFSPartitions holds all partition information for ZFS pool creation
type ZFSPartitions struct {
	Data  []string
	Cache []string
	Log   []string
	Spare []string
}

// prepareZFSPartitions converts disk IDs to partition paths
func prepareZFSPartitions(
	zfsDiskIDsCache []string,
	zfsDiskIDsLog []string,
	zfsDiskIDsData []string,
	zfsDiskIDsSpare []string,
) ZFSPartitions {
	// Prepare data partition IDs
	dataPartitions := make([]string, len(zfsDiskIDsData))
	for i, disk := range zfsDiskIDsData {
		// udev consistently creates partition links with the "-partN" suffix for by-id paths.
		dataPartitions[i] = fmt.Sprintf("%s-part1", disk)
	}

	// Prepare cache partition IDs
	cachePartitions := make([]string, len(zfsDiskIDsCache))
	for i, disk := range zfsDiskIDsCache {
		cachePartitions[i] = fmt.Sprintf("%s-part1", disk)
	}

	// Prepare log partition IDs
	logPartitions := make([]string, len(zfsDiskIDsLog))
	for i, disk := range zfsDiskIDsLog {
		logPartitions[i] = fmt.Sprintf("%s-part1", disk)
	}

	// Prepare spare partition IDs
	sparePartitions := make([]string, len(zfsDiskIDsSpare))
	for i, disk := range zfsDiskIDsSpare {
		sparePartitions[i] = fmt.Sprintf("%s-part1", disk)
	}

	return ZFSPartitions{
		Data:  dataPartitions,
		Cache: cachePartitions,
		Log:   logPartitions,
		Spare: sparePartitions,
	}
}

// buildZFSPoolArgs builds the pool creation arguments based on the pool type
func buildZFSPoolArgs(zfsPoolType string, partitions ZFSPartitions) ([]string, error) {
	var zfsPoolArgs []string

	// Build the pool creation arguments based on the pool type
	switch zfsPoolType {
	case "single":
		// Single disk pool
		log.Println("Creating single-disk ZFS pool")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Data[0])

	case "mirror":
		// Mirror pool
		log.Println("Creating mirrored ZFS pool")
		zfsPoolArgs = append(zfsPoolArgs, "mirror")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Data...)

	case "stripe":
		// Stripe pool (no special keyword needed)
		log.Println("Creating striped ZFS pool")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Data...)

	case "raidz":
		// RAID-Z pool
		log.Println("Creating RAID-Z ZFS pool")
		zfsPoolArgs = append(zfsPoolArgs, "raidz")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Data...)

	case "raidz2":
		// RAID-Z2 pool
		log.Println("Creating RAID-Z2 ZFS pool")
		zfsPoolArgs = append(zfsPoolArgs, "raidz2")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Data...)

	case "raidz3":
		// RAID-Z3 pool
		log.Println("Creating RAID-Z3 ZFS pool")
		zfsPoolArgs = append(zfsPoolArgs, "raidz3")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Data...)

	default:
		return nil, fmt.Errorf("invalid ZFS pool type: %s", zfsPoolType)
	}

	// Add special devices
	zfsPoolArgs = appendSpecialDevices(zfsPoolArgs, partitions)

	return zfsPoolArgs, nil
}

// appendSpecialDevices adds cache, log, and spare devices to pool arguments
func appendSpecialDevices(zfsPoolArgs []string, partitions ZFSPartitions) []string {
	// Add cache devices if specified
	if len(partitions.Cache) > 0 {
		log.Printf("Adding %d cache device(s) to ZFS pool", len(partitions.Cache))
		zfsPoolArgs = append(zfsPoolArgs, "cache")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Cache...)
	}

	// Add log devices if specified
	if len(partitions.Log) > 0 {
		log.Printf("Adding %d log device(s) to ZFS pool", len(partitions.Log))
		zfsPoolArgs = append(zfsPoolArgs, "log")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Log...)
	}

	// Add spare devices if specified
	if len(partitions.Spare) > 0 {
		log.Printf("Adding %d spare device(s) to ZFS pool", len(partitions.Spare))
		zfsPoolArgs = append(zfsPoolArgs, "spare")
		zfsPoolArgs = append(zfsPoolArgs, partitions.Spare...)
	}

	return zfsPoolArgs
}

// executeZFSPoolCreation executes the actual pool creation command
func executeZFSPoolCreation(
	execute bool,
	mountPoint string,
	zfsPoolName string,
	zfsPoolCompression bool,
	ashift int,
	zfsPoolArgs []string,
	partitions ZFSPartitions,
) error {
	log.Printf("Creating ZFS pool %s", zfsPoolName)
	log.Printf("Data partitions: %v", partitions.Data)
	if len(partitions.Cache) > 0 {
		log.Printf("Cache partitions: %v", partitions.Cache)
	}
	if len(partitions.Log) > 0 {
		log.Printf("Log partitions: %v", partitions.Log)
	}
	if len(partitions.Spare) > 0 {
		log.Printf("Spare partitions: %v", partitions.Spare)
	}

	// Prepare common pool arguments
	zfsPoolCreateArgs := []string{
		"create",
		"-f",
		"-o", fmt.Sprintf("ashift=%d", ashift),
		"-o", "autotrim=on",
		"-O", "acltype=posixacl",
		"-O", "relatime=on",
		"-O", "xattr=sa",
		"-O", "dnodesize=auto",
		"-O", "normalization=formD",
		"-O", "mountpoint=none",
		"-O", "canmount=off",
		"-O", "devices=off",
	}

	// Add compression if enabled (default is true)
	if zfsPoolCompression {
		zfsPoolCreateArgs = append(zfsPoolCreateArgs, "-O", "compression=zstd")
	} else {
		zfsPoolCreateArgs = append(zfsPoolCreateArgs, "-O", "compression=off")
	}

	// Set the altroot temporary mountpoint for the install
	zfsPoolCreateArgs = append(
		zfsPoolCreateArgs,
		"-R", mountPoint,
	)
	log.Printf("Setting altroot mountpoint for ZFS pool: %s", mountPoint)

	// Add pool name
	zfsPoolCreateArgs = append(zfsPoolCreateArgs, zfsPoolName)

	// Add the pool-specific arguments (data, cache, log, spare devices)
	zfsPoolCreateArgs = append(zfsPoolCreateArgs, zfsPoolArgs...)

	// DEBUG: Print the pool creation arguments
	log.Printf("ZFS pool creation arguments: %v", zfsPoolCreateArgs)

	// Execute the pool creation command
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zpool",
		zfsPoolCreateArgs...,
	)
	if err != nil {
		return fmt.Errorf("failed to create ZFS pool %s: %w", zfsPoolName, err)
	}

	return nil
}

// createZFSDatasets creates the necessary ZFS datasets on the pool.
// Returns an error if any dataset creation fails.
func createZFSDatasets(
	execute bool,
	zfsPoolName string,
	configData *config.Config,
) error {

	log.Printf("--- Creating ZFS Datasets on pool %s ---", zfsPoolName)

	// Create datasets in order
	if err := createBootDataset(execute, zfsPoolName); err != nil {
		return err
	}

	if err := createRootDataset(execute, zfsPoolName); err != nil {
		return err
	}

	if err := createBasicDatasets(execute, zfsPoolName); err != nil {
		return err
	}

	if err := createSwapDataset(execute, zfsPoolName, configData); err != nil {
		return err
	}

	if err := createSystemDatasets(execute, zfsPoolName); err != nil {
		return err
	}

	if err := createContainerDatasets(execute, zfsPoolName); err != nil {
		return err
	}

	// Wait a bit for ZFS changes to settle
	if execute {
		log.Println("Waiting for ZFS changes to settle...")
		time.Sleep(5 * time.Second)
	}

	log.Println("--- ZFS Dataset Creation Complete ---")
	return nil
}

// createBootDataset creates the boot dataset and sets bootfs property
func createBootDataset(execute bool, zfsPoolName string) error {
	zfsDatasetPathBoot := path.Join(zfsPoolName, zfsDatasetBoot)

	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathBoot)
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/boot",
		zfsDatasetPathBoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create boot ZFS dataset %s: %w", zfsDatasetPathBoot, err)
	}

	// Mount the boot dataset to set bootfs property
	err = mountZFSDataset(execute, zfsDatasetPathBoot)
	if err != nil {
		return fmt.Errorf("failed to mount boot filesystem: %w", err)
	}

	// Set the bootfs property on the boot pool
	log.Printf("Setting bootfs property on %s to %s.\n", zfsPoolName, zfsDatasetPathBoot)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zpool",
		"set",
		fmt.Sprintf("bootfs=%s", zfsDatasetPathBoot),
		zfsPoolName,
	)
	if err != nil {
		// Attempt to unmount before returning the error
		_, errUnmount := sysutil.Execute(
			execute,
			sysutil.ModeNormal,
			"zfs",
			"unmount",
			zfsDatasetPathBoot,
		)
		if errUnmount != nil {
			log.Printf(
				"Warning! Failed to unmount temporary root mount %s: %v\n",
				zfsDatasetPathBoot,
				errUnmount,
			)
		}
		return fmt.Errorf("failed to set bootfs property on %s: %w", zfsPoolName, err)
	}

	return nil
}

// createRootDataset creates the root dataset and sets bootfs property
func createRootDataset(execute bool, zfsPoolName string) error {
	zfsDatasetPathRoot := path.Join(zfsPoolName, zfsDatasetRoot)

	log.Printf("Creating ZFS dataset: %s", zfsDatasetPathRoot)
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/",
		zfsDatasetPathRoot,
	)
	if err != nil {
		return fmt.Errorf("failed to create root ZFS dataset %s: %w", zfsDatasetPathRoot, err)
	}

	// Mount the root dataset to set bootfs property
	err = mountZFSDataset(execute, zfsDatasetPathRoot)
	if err != nil {
		return fmt.Errorf("failed to mount root filesystem: %w", err)
	}

	// Set the bootfs property on the pool
	log.Printf("Setting bootfs property on %s to %s", zfsPoolName, zfsDatasetPathRoot)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zpool",
		"set",
		fmt.Sprintf("bootfs=%s", zfsDatasetPathRoot),
		zfsPoolName,
	)
	if err != nil {
		// Attempt to unmount before returning the error
		_, errUnmount := sysutil.Execute(
			execute,
			sysutil.ModeNormal,
			"zfs",
			"unmount",
			zfsDatasetPathRoot,
		)
		if errUnmount != nil {
			log.Printf(
				"Warning! Failed to unmount temporary root mount %s: %v",
				zfsDatasetPathRoot,
				errUnmount,
			)
		}
		return fmt.Errorf("failed to set bootfs property on %s: %w", zfsPoolName, err)
	}

	return nil
}

// createBasicDatasets creates home and nix store datasets
func createBasicDatasets(execute bool, zfsPoolName string) error {
	// --- Home Dataset ---
	zfsDatasetPathHome := path.Join(zfsPoolName, zfsDatasetHome)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathHome)
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/home",
		zfsDatasetPathHome,
	)
	if err != nil {
		return fmt.Errorf("failed to create home ZFS dataset %s: %w", zfsDatasetPathHome, err)
	}

	// --- Nix Store Dataset ---
	zfsDatasetPathNix := path.Join(zfsPoolName, zfsDatasetNixStore)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathNix)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/nix",
		"-o", "atime=off", // Optimization for Nix store
		zfsDatasetPathNix,
	)
	if err != nil {
		return fmt.Errorf("failed to create nix ZFS dataset %s: %w", zfsDatasetPathNix, err)
	}

	return nil
}

// createSwapDataset creates the swap dataset if enabled
func createSwapDataset(execute bool, zfsPoolName string, configData *config.Config) error {
	if !configData.Swap.Enabled {
		log.Println("Skipping swap dataset creation as it is disabled.")
		return nil
	}

	zfsDatasetPathSwap := path.Join(zfsPoolName, zfsDatasetSwap)
	log.Printf(
		"Creating ZFS swap volume: %s with size %s\n",
		zfsDatasetPathSwap,
		configData.Swap.Size,
	)
	// Use a 16k block size for swap, as recommended by ZFS documentation
	// for better performance and to avoid wasted space.
	const swapVolBlockSize = "16k"

	// Create the swap ZFS volume
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-V",
		configData.Swap.Size,
		"-b",
		swapVolBlockSize,
		"-o", "compression=zle", // Different compression for swap
		"-o", "logbias=throughput",
		"-o", "sync=always",
		"-o", "primarycache=metadata",
		"-o", "secondarycache=none",
		"-o", "com.sun:auto-snapshot=false", // Don't snapshot swap
		zfsDatasetPathSwap,
	)
	if err != nil {
		return fmt.Errorf("failed to create swap ZFS volume %s: %w", zfsDatasetPathSwap, err)
	}

	return formatSwapDevice(execute, zfsDatasetPathSwap, zfsPoolName)
}

// formatSwapDevice handles swap device detection and formatting
func formatSwapDevice(execute bool, zfsDatasetPathSwap, zfsPoolName string) error {
	if !execute {
		return nil
	}

	// Wait for zvol device to appear
	swapDevicePath := fmt.Sprintf("/dev/zvol/%s", zfsDatasetPathSwap)
	log.Printf("Waiting for swap device to appear at %s", swapDevicePath)

	// Try a few times with exponential backoff
	for attempt := 1; attempt <= 5; attempt++ {
		if _, err := os.Stat(swapDevicePath); err == nil {
			log.Printf("Swap device found after %d attempts", attempt)
			break
		}

		// Trigger udev to reload devices
		_, err := sysutil.Execute(execute, sysutil.ModeNormal, "udevadm", "trigger")
		if err != nil {
			log.Printf("Warning: udevadm trigger failed: %v", err)
		}
		_, err = sysutil.Execute(execute, sysutil.ModeNormal, "udevadm", "settle")
		if err != nil {
			log.Printf("Warning: udevadm settle failed: %v", err)
		}

		// Check if the directory exists, create if needed
		dir := path.Dir(swapDevicePath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			log.Printf("Creating directory: %s", dir)
			if err := os.MkdirAll(dir, 0750); err != nil {
				log.Printf("Warning: Failed to create directory %s: %v", dir, err)
			}
		}

		waitTime := time.Duration(attempt*2) * time.Second
		log.Printf(
			"Waiting %v seconds for swap device (attempt %d/5)...",
			waitTime.Seconds(),
			attempt,
		)
		time.Sleep(waitTime)
	}

	// Final verification with alternative paths
	if _, err := os.Stat(swapDevicePath); os.IsNotExist(err) {
		// Try alternative paths as fallback
		alternativePaths := []string{
			"/dev/zd0", // Sometimes used for first zvol
			fmt.Sprintf("/dev/%s/%s", zfsPoolName, "swap"), // Alternative path format
		}

		for _, altPath := range alternativePaths {
			log.Printf("Checking alternative swap path: %s", altPath)
			if _, err := os.Stat(altPath); err == nil {
				swapDevicePath = altPath
				log.Printf("Using alternative swap device path: %s", swapDevicePath)
				break
			}
		}
	}

	// Then format swap using the confirmed path
	log.Printf("Formatting swap volume: %s\n", swapDevicePath)
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mkswap",
		swapDevicePath,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to format swap volume %s: %w",
			swapDevicePath,
			err,
		)
	}

	return nil
}

// createSystemDatasets creates tmp, usr, and var datasets
func createSystemDatasets(execute bool, zfsPoolName string) error {
	// --- Tmp Dataset ---
	zfsDatasetPathTmp := path.Join(zfsPoolName, zfsDatasetTmp)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathTmp)
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/tmp",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathTmp,
	)
	if err != nil {
		return fmt.Errorf("failed to create tmp ZFS dataset %s: %w", zfsDatasetPathTmp, err)
	}

	// --- Var Dataset ---
	zfsDatasetPathVar := path.Join(zfsPoolName, zfsDatasetVar)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathVar)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var",
		zfsDatasetPathVar,
	)
	if err != nil {
		return fmt.Errorf("failed to create var ZFS dataset %s: %w", zfsDatasetPathVar, err)
	}

	// --- Var/Lib Dataset ---
	zfsDatasetPathLib := path.Join(zfsPoolName, zfsDatasetLib)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathLib)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var/lib",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathLib,
	)
	if err != nil {
		return fmt.Errorf("failed to create lib ZFS dataset %s: %w", zfsDatasetPathLib, err)
	}

	return nil
}

// createContainerDatasets creates Docker, containers, and Incus datasets
func createContainerDatasets(execute bool, zfsPoolName string) error {
	// --- Var/Lib/Docker Dataset ---
	zfsDatasetPathDocker := path.Join(zfsPoolName, zfsDatasetDocker)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathDocker)
	_, err := sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var/lib/docker",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathDocker,
	)
	if err != nil {
		return fmt.Errorf("failed to create docker ZFS dataset %s: %w", zfsDatasetPathDocker, err)
	}

	// --- Var/Lib/Containers Dataset ---
	zfsDatasetPathContainers := path.Join(zfsPoolName, zfsDatasetContainers)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathContainers)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var/lib/containers",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathContainers,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to create containers ZFS dataset %s: %w",
			zfsDatasetPathContainers,
			err,
		)
	}

	// --- Var/Lib/Incus Dataset ---
	zfsDatasetPathIncus := path.Join(zfsPoolName, zfsDatasetIncus)
	log.Printf("Creating ZFS dataset: %s\n", zfsDatasetPathIncus)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"create",
		"-o", "canmount=on",
		"-o", "mountpoint=/var/lib/incus",
		"-o", "com.sun:auto-snapshot=false",
		zfsDatasetPathIncus,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to create incus ZFS dataset %s: %w",
			zfsDatasetPathIncus,
			err,
		)
	}

	return nil
}
