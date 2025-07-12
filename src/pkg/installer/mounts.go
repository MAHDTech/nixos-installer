package installer

import (
	"fmt"
	"path"
	"strings"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	sysutil "github.com/MAHDTech/nixos-installer/pkg/sysutil"
)

// MountFileSystems mounts the created partitions and datasets
// to their target directories.
func mountFileSystems(
	execute bool,
	mountPoint string,
	configData *config.Config,
	partitionInfo PartitionInfo,
	zfsPoolName string,
) error {
	sysutil.Info("Mounting filesystems...")

	// Define ZFS dataset paths
	zfsDatasetPathBoot := path.Join(zfsPoolName, zfsDatasetBoot)
	zfsDatasetPathRoot := path.Join(zfsPoolName, zfsDatasetRoot)
	zfsDatasetPathHome := path.Join(zfsPoolName, zfsDatasetHome)
	zfsDatasetPathNix := path.Join(zfsPoolName, zfsDatasetNixStore)
	zfsDatasetPathVar := path.Join(zfsPoolName, zfsDatasetVar)
	zfsDatasetPathLib := path.Join(zfsPoolName, zfsDatasetLib)
	zfsDatasetPathDocker := path.Join(zfsPoolName, zfsDatasetDocker)
	zfsDatasetPathContainers := path.Join(zfsPoolName, zfsDatasetContainers)
	zfsDatasetPathIncus := path.Join(zfsPoolName, zfsDatasetIncus)
	zfsDatasetPathIncusStoragePools := path.Join(zfsPoolName, zfsDatasetIncusStoragePools)
	zfsDatasetPathTmp := path.Join(zfsPoolName, zfsDatasetTmp)

	// Mount core filesystems
	if err := mountCoreFilesystems(execute, mountPoint, partitionInfo, zfsDatasetPathRoot, zfsDatasetPathBoot); err != nil {
		return err
	}

	// Mount NixOS config partition if enabled
	if err := mountNixOSConfigPartition(execute, mountPoint, configData, partitionInfo); err != nil {
		return err
	}

	// Mount user filesystems
	if err := mountUserFilesystems(execute, zfsDatasetPathHome, zfsDatasetPathNix); err != nil {
		return err
	}

	// Mount system filesystems
	if err := mountSystemFilesystems(execute, zfsDatasetPathVar, zfsDatasetPathLib); err != nil {
		return err
	}

	// Mount container filesystems
	if err := mountContainerFilesystems(execute, zfsDatasetPathDocker, zfsDatasetPathContainers, zfsDatasetPathIncus, zfsDatasetPathIncusStoragePools); err != nil {
		return err
	}

	// Mount temporary filesystem
	if err := mountTemporaryFilesystem(execute, mountPoint, zfsDatasetPathTmp); err != nil {
		return err
	}

	sysutil.Info("All filesystems mounted!")
	return nil
}

// mountCoreFilesystems mounts the root and boot filesystems
func mountCoreFilesystems(
	execute bool,
	mountPoint string,
	partitionInfo PartitionInfo,
	zfsDatasetPathRoot, zfsDatasetPathBoot string,
) error {
	// 1. Mount the root dataset to the configured altroot
	err := mountZFSDataset(execute, zfsDatasetPathRoot)
	if err != nil {
		return fmt.Errorf("failed to mount root filesystem: %w", err)
	}

	// 2. Mount the boot pool to the configured altroot
	err = mountZFSDataset(execute, zfsDatasetPathBoot)
	if err != nil {
		return fmt.Errorf("failed to mount boot filesystem: %w", err)
	}

	// 3. Mount the UEFI partition to "/boot/efi"
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	sysutil.Info("Mounting UEFI partition %s to %s.", partitionInfo.UEFI, mountPointUEFI)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"mount",
		"-o",
		"X-mount.mkdir", // Option to create dir if it doesn't exist
		"-t",
		"vfat", // Filesystem type for UEFI
		partitionInfo.UEFI,
		mountPointUEFI,
	)
	if err != nil {
		return fmt.Errorf("failed to mount UEFI partition: %w", err)
	}

	return nil
}

// mountNixOSConfigPartition mounts the NixOS config partition if enabled
func mountNixOSConfigPartition(
	execute bool,
	mountPoint string,
	configData *config.Config,
	partitionInfo PartitionInfo,
) error {
	// Mount the NixOS config partition if enabled.
	mountPointNixOSConfig := path.Join(mountPoint, "boot/nixos")
	if configData.NixOS.Config.Enabled {
		sysutil.Info(
			"Mounting NixOS config partition %s to %s.",
			partitionInfo.NixOSConfig,
			mountPointNixOSConfig,
		)
		_, err := sysutil.Execute(
			execute,
			sysutil.ModeNormal,
			"mount",
			"-o",
			"X-mount.mkdir",
			"-t",
			"xfs", // Filesystem type used during partitioning
			partitionInfo.NixOSConfig,
			mountPointNixOSConfig,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to mount NixOS config partition %s: %w",
				partitionInfo.NixOSConfig,
				err,
			)
		}
	} else {
		sysutil.Info("Skipping NixOS config partition mounting as it is disabled.")
	}

	return nil
}

// mountUserFilesystems mounts user-related filesystems
func mountUserFilesystems(execute bool, zfsDatasetPathHome, zfsDatasetPathNix string) error {
	// Mount the home dataset
	err := mountZFSDataset(execute, zfsDatasetPathHome)
	if err != nil {
		return fmt.Errorf("failed to mount home filesystem: %w", err)
	}

	// Mount the nix dataset
	err = mountZFSDataset(execute, zfsDatasetPathNix)
	if err != nil {
		return fmt.Errorf("failed to mount nix filesystem: %w", err)
	}

	return nil
}

// mountSystemFilesystems mounts system-related filesystems
func mountSystemFilesystems(execute bool, zfsDatasetPathVar, zfsDatasetPathLib string) error {
	// Mount the var dataset
	err := mountZFSDataset(execute, zfsDatasetPathVar)
	if err != nil {
		return fmt.Errorf("failed to mount var filesystem: %w", err)
	}

	// Mount the lib dataset
	err = mountZFSDataset(execute, zfsDatasetPathLib)
	if err != nil {
		return fmt.Errorf("failed to mount lib filesystem: %w", err)
	}

	return nil
}

// mountContainerFilesystems mounts container-related filesystems
func mountContainerFilesystems(
	execute bool,
	zfsDatasetPathDocker, zfsDatasetPathContainers, zfsDatasetPathIncus, zfsDatasetPathIncusStoragePools string,
) error {
	// Mount the docker dataset
	err := mountZFSDataset(execute, zfsDatasetPathDocker)
	if err != nil {
		return fmt.Errorf("failed to mount docker filesystem: %w", err)
	}

	// Mount the containers dataset
	err = mountZFSDataset(execute, zfsDatasetPathContainers)
	if err != nil {
		return fmt.Errorf("failed to mount containers filesystem: %w", err)
	}

	// Mount the incus dataset
	err = mountZFSDataset(execute, zfsDatasetPathIncus)
	if err != nil {
		return fmt.Errorf("failed to mount incus filesystem: %w", err)
	}

	// Mount the incus storage pools dataset
	err = mountZFSDataset(execute, zfsDatasetPathIncusStoragePools)
	if err != nil {
		return fmt.Errorf("failed to mount incus storage pools filesystem: %w", err)
	}

	return nil
}

// mountTemporaryFilesystem mounts the temporary filesystem
func mountTemporaryFilesystem(execute bool, mountPoint, zfsDatasetPathTmp string) error {
	// Mount the tmp dataset
	mountPointTmp := path.Join(mountPoint, "tmp")
	err := mountZFSDataset(execute, zfsDatasetPathTmp)
	if err != nil {
		return fmt.Errorf("failed to mount tmp filesystem: %w", err)
	}

	// Set permissions for /tmp
	sysutil.Info("Setting permissions for %s", mountPointTmp)
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"chmod",
		"1777",
		mountPointTmp,
	)
	if err != nil {
		return fmt.Errorf("failed to set permissions for /tmp: %w", err)
	}

	return nil
}

// mountZFSDataset ensures a ZFS dataset is mounted with its configured mountpoint
// When altroot is used, ZFS will automatically use altroot + dataset mountpoint
func mountZFSDataset(execute bool, dataset string) error {
	sysutil.Info("Mount ZFS dataset %s to configured altroot", dataset)

	// Check if the dataset is mounted
	mountedOutput, err := sysutil.Execute(
		execute,
		sysutil.ModeStdOut,
		"zfs",
		"get",
		"-H",
		"-o", "value",
		"mounted",
		dataset,
	)

	// If already mounted, we're done here.
	if err == nil && strings.TrimSpace(mountedOutput) == "yes" {
		sysutil.Info("Dataset %s is already mounted", dataset)
		return nil
	}

	// Mount the dataset to the configured altroot + location
	_, err = sysutil.Execute(
		execute,
		sysutil.ModeNormal,
		"zfs",
		"mount",
		dataset,
	)
	if err != nil {
		return fmt.Errorf("failed to mount dataset %s: %w", dataset, err)
	}

	return nil
}
