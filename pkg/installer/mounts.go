package installer

import (
	"fmt"
	"log"
	"path"
	"strings"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
)

// MountFileSystems mounts the created partitions and datasets
// to their target directories.
func mountFileSystems(
	execute bool,
	mountPoint string,
	configData *config.Config,
	partitionInfo PartitionInfo,
	zfsPoolBootName string,
	zfsPoolRootName string,
) error {
	log.Println("Mounting filesystems...")

	// Define ZFS dataset paths for the boot pool
	zfsDataSetPathBoot := path.Join(zfsPoolBootName, zfsDatasetBoot)

	// Define ZFS dataset paths for the root pool
	zfsDataSetPathRoot := path.Join(zfsPoolRootName, zfsDatasetRoot)
	zfsDataSetPathHome := path.Join(zfsPoolRootName, zfsDatasetHome)
	zfsDataSetPathNix := path.Join(zfsPoolRootName, zfsDatasetNixStore)
	zfsDataSetPathVar := path.Join(zfsPoolRootName, zfsDatasetVar)
	zfsDataSetPathLib := path.Join(zfsPoolRootName, zfsDatasetLib)
	zfsDataSetPathDocker := path.Join(zfsPoolRootName, zfsDatasetDocker)
	zfsDataSetPathContainers := path.Join(zfsPoolRootName, zfsDatasetContainers)
	zfsDataSetPathTmp := path.Join(zfsPoolRootName, zfsDatasetTmp)

	// 1. Mount the root dataset to the configured altroot
	//    Example: /mnt/nixos + "/"
	err := mountZFSDataset(execute, zfsDataSetPathRoot)
	if err != nil {
		return fmt.Errorf("failed to mount root filesystem: %w", err)
	}

	// 2. Mount the boot pool to the configured altroot
	//    Example: /mnt/nixos + "/boot"
	err = mountZFSDataset(execute, zfsDataSetPathBoot)
	if err != nil {
		return fmt.Errorf("failed to mount boot filesystem: %w", err)
	}

	// 3. Mount the UEFI partition to "/boot/efi"
	//    Example: /mnt/nixos + "/boot/efi"
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
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
	//    Example: /mnt/nixos + "/boot/nixos-config"
	mountPointNixOSConfig := path.Join(mountPoint, "boot/nixos-config")
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

	// 5. Mount the home dataset to the configured altroot
	//    Example: /mnt/nixos + "/home"
	err = mountZFSDataset(execute, zfsDataSetPathHome)
	if err != nil {
		return fmt.Errorf("failed to mount home filesystem: %w", err)
	}

	// 6. Mount the nix dataset to the configured altroot
	//    Example: /mnt/nixos + "/nix"
	err = mountZFSDataset(execute, zfsDataSetPathNix)
	if err != nil {
		return fmt.Errorf("failed to mount nix filesystem: %w", err)
	}

	// 7. Mount the var dataset to the configured altroot
	//    Example: /mnt/nixos + "/var"
	//    This dataset has canmount=off, so we mount the children datasets.
	err = mountZFSDataset(execute, zfsDataSetPathVar)
	if err != nil {
		return fmt.Errorf("failed to mount var filesystem: %w", err)
	}

	// 8. Mount the lib dataset to the configured altroot
	//    Example: /mnt/nixos + "/var/lib"
	err = mountZFSDataset(execute, zfsDataSetPathLib)
	if err != nil {
		return fmt.Errorf("failed to mount lib filesystem: %w", err)
	}

	// 9. Mount the docker dataset to the configured altroot
	//    Example: /mnt/nixos + "/var/lib/docker"
	err = mountZFSDataset(execute, zfsDataSetPathDocker)
	if err != nil {
		return fmt.Errorf("failed to mount docker filesystem: %w", err)
	}

	// 10. Mount the containers dataset to the configured altroot
	//     Example: /mnt/nixos + "/var/lib/containers"
	err = mountZFSDataset(execute, zfsDataSetPathContainers)
	if err != nil {
		return fmt.Errorf("failed to mount containers filesystem: %w", err)
	}

	// 11. Mount the tmp dataset to the configured altroot
	//     Example: /mnt/nixos + "/tmp"
	mountPointTmp := path.Join(mountPoint, "tmp")
	err = mountZFSDataset(execute, zfsDataSetPathTmp)
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

// mountZFSDataset ensures a ZFS dataset is mounted with its configured mountpoint
// When altroot is used, ZFS will automatically use altroot + dataset mountpoint
func mountZFSDataset(execute bool, dataset string) error {
	log.Printf("Mount ZFS dataset %s to configured altroot", dataset)

	// Check if the dataset is mounted
	mountedOutput, err := utils.Execute(
		execute,
		utils.ModeStdOut,
		"zfs",
		"get",
		"-H",
		"-o", "value",
		"mounted",
		dataset,
	)

	// If already mounted, we're done
	if err == nil && strings.TrimSpace(mountedOutput) == "yes" {
		log.Printf("Dataset %s is already mounted", dataset)
		return nil
	}

	// Mount the dataset to the configured altroot + location
	_, err = utils.Execute(
		execute,
		utils.ModeNormal,
		"zfs",
		"mount",
		dataset,
	)

	return err
}
