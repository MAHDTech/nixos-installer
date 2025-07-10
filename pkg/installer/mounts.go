package installer

import (
	"fmt"
	"log"
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
	log.Println("Mounting filesystems...")

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
	zfsDatasetPathTmp := path.Join(zfsPoolName, zfsDatasetTmp)

	// 1. Mount the root dataset to the configured altroot
	//    Example: /mnt/nixos + "/"
	err := mountZFSDataset(execute, zfsDatasetPathRoot)
	if err != nil {
		return fmt.Errorf("failed to mount root filesystem: %w", err)
	}

	// 2. Mount the boot pool to the configured altroot
	//    Example: /mnt/nixos + "/boot"
	err = mountZFSDataset(execute, zfsDatasetPathBoot)
	if err != nil {
		return fmt.Errorf("failed to mount boot filesystem: %w", err)
	}

	// 3. Mount the UEFI partition to "/boot/efi"
	//    Example: /mnt/nixos + "/boot/efi"
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	log.Printf("Mounting UEFI partition %s to %s.\n", partitionInfo.UEFI, mountPointUEFI)
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

	// 4. Mount the NixOS config partition if enabled.
	//    Example: /mnt/nixos + "/boot/nixos"
	mountPointNixOSConfig := path.Join(mountPoint, "boot/nixos")
	if configData.NixOS.Config.Enabled {
		log.Printf(
			"Mounting NixOS config partition %s to %s.\n",
			partitionInfo.NixOSConfig,
			mountPointNixOSConfig,
		)
		_, err = sysutil.Execute(
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
		log.Println("Skipping NixOS config partition mounting as it is disabled.")
	}

	// 5. Mount the home dataset to the configured altroot
	//    Example: /mnt/nixos + "/home"
	err = mountZFSDataset(execute, zfsDatasetPathHome)
	if err != nil {
		return fmt.Errorf("failed to mount home filesystem: %w", err)
	}

	// 6. Mount the nix dataset to the configured altroot
	//    Example: /mnt/nixos + "/nix"
	err = mountZFSDataset(execute, zfsDatasetPathNix)
	if err != nil {
		return fmt.Errorf("failed to mount nix filesystem: %w", err)
	}

	// 7. Mount the var dataset to the configured altroot
	//    Example: /mnt/nixos + "/var"
	err = mountZFSDataset(execute, zfsDatasetPathVar)
	if err != nil {
		return fmt.Errorf("failed to mount var filesystem: %w", err)
	}

	// 8. Mount the lib dataset to the configured altroot
	//    Example: /mnt/nixos + "/var/lib"
	err = mountZFSDataset(execute, zfsDatasetPathLib)
	if err != nil {
		return fmt.Errorf("failed to mount lib filesystem: %w", err)
	}

	// 9. Mount the docker dataset to the configured altroot
	//    Example: /mnt/nixos + "/var/lib/docker"
	err = mountZFSDataset(execute, zfsDatasetPathDocker)
	if err != nil {
		return fmt.Errorf("failed to mount docker filesystem: %w", err)
	}

	// 10. Mount the containers dataset to the configured altroot
	//     Example: /mnt/nixos + "/var/lib/containers"
	err = mountZFSDataset(execute, zfsDatasetPathContainers)
	if err != nil {
		return fmt.Errorf("failed to mount containers filesystem: %w", err)
	}

	// 11. Mount the incus dataset to the configured altroot
	//     Example: /mnt/nixos + "/var/lib/incus"
	err = mountZFSDataset(execute, zfsDatasetPathIncus)
	if err != nil {
		return fmt.Errorf("failed to mount incus filesystem: %w", err)
	}

	// 12. Mount the tmp dataset to the configured altroot
	//     Example: /mnt/nixos + "/tmp"
	mountPointTmp := path.Join(mountPoint, "tmp")
	err = mountZFSDataset(execute, zfsDatasetPathTmp)
	if err != nil {
		return fmt.Errorf("failed to mount tmp filesystem: %w", err)
	}

	// 13. Set permissions for /tmp
	log.Printf("Setting permissions for %s\n", mountPointTmp)
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

	log.Println("All filesystems mounted!")
	return nil
}

// mountZFSDataset ensures a ZFS dataset is mounted with its configured mountpoint
// When altroot is used, ZFS will automatically use altroot + dataset mountpoint
func mountZFSDataset(execute bool, dataset string) error {
	log.Printf("Mount ZFS dataset %s to configured altroot", dataset)

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
		log.Printf("Dataset %s is already mounted", dataset)
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
