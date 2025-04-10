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
	partitionInfo PartitionInfo,
	zfsPoolBootName string,
	zfsPoolRootName string,
) error {
	log.Println("Mounting filesystems...")

	// Mount point for root is just mountPoint itself, handled by zfs create/mount

	// Define mount points (boot pool)
	mountPointBoot := path.Join(mountPoint, "boot")

	// Define mount points (UEFI)
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	mountPointNixOSConfig := path.Join(mountPoint, "boot/nixos-config")

	// Define mount points (root pool)
	mountPointHome := path.Join(mountPoint, "home")
	mountPointNix := path.Join(mountPoint, "nix")
	mountPointVar := path.Join(mountPoint, "var")
	mountPointLib := path.Join(mountPoint, "var/lib")
	mountPointDocker := path.Join(mountPoint, "var/lib/docker")
	mountPointContainers := path.Join(mountPoint, "var/lib/containers")
	mountPointTmp := path.Join(mountPoint, "tmp")

	// Define ZFS dataset paths (boot pool)
	zfsDataSetPathBoot := path.Join(zfsPoolBootName, zfsDatasetBoot)

	// Define ZFS dataset paths (root pool)
	zfsDataSetPathRoot := path.Join(zfsPoolRootName, zfsDatasetRoot)
	zfsDataSetPathHome := path.Join(zfsPoolRootName, zfsDatasetHome)
	zfsDataSetPathNix := path.Join(zfsPoolRootName, zfsDatasetNixStore)
	zfsDataSetPathVar := path.Join(zfsPoolRootName, zfsDatasetVar)
	zfsDataSetPathLib := path.Join(zfsPoolRootName, zfsDatasetLib)
	zfsDataSetPathDocker := path.Join(zfsPoolRootName, zfsDatasetDocker)
	zfsDataSetPathContainers := path.Join(zfsPoolRootName, zfsDatasetContainers)
	zfsDataSetPathTmp := path.Join(zfsPoolRootName, zfsDatasetTmp)

	// 1. Mount the underlying root dataset to the mount point
	//    Example: /mnt/nixos
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathRoot, mountPoint)
	_, err := utils.Execute(
		execute,
		utils.ModeNormal,
		"mount",
		"-t",
		"zfs",
		zfsDataSetPathRoot,
		mountPoint,
	)
	if err != nil {
		return fmt.Errorf("failed to mount root filesystem: %w", err)
	}

	// 2. Mount the boot pool.
	//    Example: /mnt/nixos/boot
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathBoot, mountPointBoot)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"mount",
		"-t",
		"zfs",
		zfsDataSetPathBoot,
		mountPointBoot,
	)

	// 3. Mount the UEFI partition.
	//    Example: /mnt/nixos/boot/efi
	log.Printf("Mounting UEFI partition %s to %s.\n", partitionInfo.UEFI, mountPointUEFI)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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

	// 4. Mount the NixOS config partition if enabled.
	//    Example: /mnt/nixos/boot/nixos-config
	if configData.NixOS.Config.Enabled {
		log.Printf(
			"Mounting NixOS config partition %s to %s.\n",
			partitionInfo.NixOSConfig,
			mountPointNixOSConfig,
		)
		_, err = utils.Execute(
			execute,
			utils.ModeNormal,
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
		log.Println("Skipping NixOS config partition mounting as it is disabled.")
	}

	// 5. Mount the home dataset.
	//    Example: /mnt/nixos/home
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathHome, mountPointHome)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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

	// 6. Mount the nix dataset.
	//    Example: /mnt/nixos/nix
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathNix, mountPointNix)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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

	// 7. Mount the var dataset
	//    Example: /mnt/nixos/var
	//    This dataset has canmount=off, so we mount the children datasets.
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathVar, mountPointVar)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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

	// 8. Mount the lib dataset.
	//    Example: /mnt/nixos/var/lib
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathLib, mountPointLib)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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

	// 9. Mount the docker dataset.
	//    Example: /mnt/nixos/var/lib/docker
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathDocker, mountPointDocker)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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

	// 10. Mount the containers dataset.
	//     Example: /mnt/nixos/var/lib/containers
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathContainers, mountPointContainers)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathContainers,
		mountPointContainers,
	)
	if err != nil {
		return fmt.Errorf("failed to mount containers filesystem: %w", err)
	}

	// 11. Mount the tmp dataset.
	//     Example: /mnt/nixos/tmp
	log.Printf("Mounting ZFS dataset %s to %s.\n", zfsDataSetPathTmp, mountPointTmp)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
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

	// 12. Set permissions for /tmp
	log.Printf("Setting permissions for %s\n", mountPointTmp)
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"chmod",
		"1777",
		mountPointTmp,
	)
	if err != nil {
		return fmt.Errorf("failed to set permissions for /tmp: %w", err)
	}

	log.Println("All filesystems mounted!")
	return nil
}
