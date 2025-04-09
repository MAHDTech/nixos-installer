package installer

import (
	"fmt"
	"log"
	"path"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// MountFileSystems mounts the created partitions and datasets to their target directories.
func mountFileSystems(
	execute bool,
	mountPoint string,
	configData *config.Config,
	partitionNameUEFI, partitionNameNixOSConfig, zfsPoolRootName string,
) error {
	log.Println("Mounting filesystems...")

	// Define mount points based on the base mountPoint
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	mountPointNixOSConfig := path.Join(mountPoint, "boot/nixos")
	mountPointHome := path.Join(mountPoint, "home")
	mountPointNix := path.Join(mountPoint, "nix")
	mountPointVar := path.Join(mountPoint, "var")
	mountPointLib := path.Join(mountPoint, "var/lib")
	mountPointDocker := path.Join(mountPoint, "var/lib/docker")
	mountPointTmp := path.Join(mountPoint, "tmp")
	// Mount point for root is just mountPoint itself, handled by zfs create/mount

	// Define ZFS dataset paths
	zfsDataSetPathRoot := path.Join(zfsPoolRootName, zfsDatasetRoot)
	zfsDataSetPathHome := path.Join(zfsPoolRootName, zfsDatasetHome)
	zfsDataSetPathNix := path.Join(zfsPoolRootName, zfsDatasetNixStore)
	zfsDataSetPathVar := path.Join(zfsPoolRootName, zfsDatasetVar)
	zfsDataSetPathLib := path.Join(zfsPoolRootName, zfsDatasetLib)
	zfsDataSetPathDocker := path.Join(zfsPoolRootName, zfsDatasetDocker)
	zfsDataSetPathTmp := path.Join(zfsPoolRootName, zfsDatasetTmp)
	// Boot dataset/pool is not explicitly mounted here in the original script, seems handled by NixOS config generation?

	// Mount the root dataset first (was mounted temporarily before, needs proper mount)
	log.Printf("Mounting ZFS root %s to %s.\n", zfsDataSetPathRoot, mountPoint)
	err := utils.Execute(
		execute,
		"mount",
		"-t",
		"zfs",
		zfsDataSetPathRoot,
		mountPoint,
	)
	if err != nil {
		return fmt.Errorf("failed to mount root filesystem: %w", err)
	}

	// Mount the UEFI partition.
	log.Printf("Mounting UEFI partition %s to %s.\n", partitionNameUEFI, mountPointUEFI)
	err = utils.Execute(
		execute,
		"mount",
		"-o",
		"X-mount.mkdir", // Option to create dir if it doesn't exist (should exist from prepare.go)
		"-t",
		"vfat", // Filesystem type for UEFI
		partitionNameUEFI,
		mountPointUEFI,
	)
	if err != nil {
		return fmt.Errorf("failed to mount UEFI partition: %w", err)
	}

	// Mount the NixOS config partition if enabled.
	if configData.NixOS.Config.Enabled {
		log.Printf(
			"Mounting NixOS config partition %s to %s.\n",
			partitionNameNixOSConfig,
			mountPointNixOSConfig,
		)
		err = utils.Execute(
			execute,
			"mount",
			"-o",
			"X-mount.mkdir",
			"-t",
			"xfs", // Filesystem type used during partitioning
			partitionNameNixOSConfig,
			mountPointNixOSConfig,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to mount NixOS config partition %s: %w",
				partitionNameNixOSConfig,
				err,
			)
		}
	} else {
		log.Println("Skipping NixOS config partition mounting as it is disabled.")
	}

	// Mount the home dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathHome, mountPointHome)
	err = utils.Execute(
		execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathHome,
		mountPointHome,
	)
	if err != nil {
		return fmt.Errorf("failed to mount home filesystem: %w", err)
	}

	// Mount the nix dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathNix, mountPointNix)
	err = utils.Execute(
		execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathNix,
		mountPointNix,
	)
	if err != nil {
		return fmt.Errorf("failed to mount nix filesystem: %w", err)
	}

	// Mount the var dataset (which has canmount=off, so we mount children).
	// The original script mounts 'var' itself, which might rely on ZFS auto-mounting children if properties are set right.
	// Let's stick to mounting children explicitly based on dataset creation.
	log.Printf(
		"Mounting %s to %s.\n",
		zfsDataSetPathVar,
		mountPointVar,
	) // This might not be needed if children are mounted? Let's keep it for now matching original.
	err = utils.Execute(
		execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathVar, // Mount the parent 'var' dataset
		mountPointVar,
	)
	if err != nil {
		return fmt.Errorf("failed to mount var filesystem: %w", err)
	}

	// Mount the lib dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathLib, mountPointLib)
	err = utils.Execute(
		execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathLib,
		mountPointLib,
	)
	if err != nil {
		return fmt.Errorf("failed to mount lib filesystem: %w", err)
	}

	// Mount the docker dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathDocker, mountPointDocker)
	err = utils.Execute(
		execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathDocker,
		mountPointDocker,
	)
	if err != nil {
		return fmt.Errorf("failed to mount docker filesystem: %w", err)
	}

	// Mount the tmp dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathTmp, mountPointTmp)
	err = utils.Execute(
		execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathTmp,
		mountPointTmp,
	)
	if err != nil {
		return fmt.Errorf("failed to mount tmp filesystem: %w", err)
	}

	// Set permissions for /tmp
	log.Printf("Setting permissions for %s\n", mountPointTmp)
	err = utils.Execute(
		execute,
		"chmod",
		"1777",
		mountPointTmp,
	)
	if err != nil {
		return fmt.Errorf("failed to set permissions for /tmp: %w", err)
	}

	log.Println("Filesystems mounted.")
	return nil
}
