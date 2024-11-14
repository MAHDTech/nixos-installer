// Package installer contains the logic for installing NixOS.
package installer

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"time"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
	validate "github.com/MAHDTech/nixos-installer/pkg/validate"
)

// Where nixos will be installed to.
const mountPoint = "/mnt/nixos"

// The names of the ZFS datasets.
const zfsDatasetBoot = "boot"
const zfsDatasetRoot = "root"
const zfsDatasetHome = "home"
const zfsDatasetNixStore = "nix"
const zfsDatasetSwap = "swap"
const zfsDatasetTmp = "tmp"
const zfsDatasetVar = "var"
const zfsDatasetLib = "var/lib"
const zfsDatasetDocker = "var/lib/docker"

// Run function is where the installer logic is executed.
func Run() {

	// Determine the path to the configuration file.
	configFile := flag.String(
		"config",
		"config.yaml",
		"Path to the YAML configuration file.",
	)

	// By default we run in dry run mode unless the 'dry-run' flag is set to false
	// to avoid a user accidentally making changes.
	execute := flag.Bool(
		"run",
		false,
		"Execute mode. (default is false which only dry runs commands)",
	)

	// By default only the disk partitioning and nix generation is done.
	// To automatically install NixOS, set the 'install' flag to true.
	executeInstall := flag.Bool(
		"install",
		false,
		"Automatically install NixOS. (default is false which only generates the NixOS configuration)",
	)

	// Parse the flags.
	flag.Parse()

	if *execute {
		log.Println("Running in execute mode.")
	} else {
		log.Println("Running in dry run mode, see '-help' for more information.")
	}

	// Read the YAML configuration file and parse it into a Config struct.
	configData, err := config.ReadConfig(*configFile)
	validate.Error(err)

	/*
		##################################################
			Mountpoints
		##################################################
	*/

	// Capture all mountpoints from stdout
	mountpointsString := utils.ExecuteStdOut(
		true,
		"lsblk",
		"--noheadings",
		"--json",
		"--output",
		"ID,MOUNTPOINTS",
	)
	// Convert the string into JSON
	mountpointsJSON := []byte(mountpointsString)

	/*
		##################################################
			Create directories
		##################################################
	*/

	// Create the directories where the temporary mount points will be created.
	log.Printf("Creating mount directory %s\n", mountPoint)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPoint,
	)

	// Create mount point for 'boot'
	mountPointBoot := path.Join(mountPoint, "boot")
	log.Printf("Creating mount point for 'boot' at: %s\n", mountPointBoot)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointBoot,
	)

	// Create mount point for 'efi'
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	log.Printf("Creating mount point for 'efi' at: %s\n", mountPointUEFI)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointUEFI,
	)

	// Create mount point for 'nixos' configuration.
	mountPointNixOSConfig := path.Join(mountPoint, "boot/nixos")
	log.Printf("Creating mount point for 'nixos-config' at: %s\n", mountPointNixOSConfig)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointNixOSConfig,
	)

	// Create mount point for 'home'
	mountPointHome := path.Join(mountPoint, "home")
	log.Printf("Creating mount point for 'home' at: %s\n", mountPointHome)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointHome,
	)

	// Create mount point for 'nix'
	mountPointNix := path.Join(mountPoint, "nix")
	log.Printf("Creating mount point for 'nix' at: %s\n", mountPointNix)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointNix,
	)

	// Create mount point for 'var'
	mountPointVar := path.Join(mountPoint, "var")
	log.Printf("Creating mount point for 'var' at: %s\n", mountPointVar)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointVar,
	)

	// Create mount point for 'lib'
	mountPointLib := path.Join(mountPoint, "var/lib")
	log.Printf("Creating mount point for 'var' at: %s\n", mountPointLib)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointLib,
	)

	// Create mount point for 'docker'
	mountPointDocker := path.Join(mountPoint, "var/lib/docker")
	log.Printf("Creating mount point for 'var' at: %s\n", mountPointDocker)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointDocker,
	)

	// Create mount point for 'tmp'
	mountPointTmp := path.Join(mountPoint, "tmp")
	log.Printf("Creating mount point for 'tmp' at: %s\n", mountPointTmp)
	utils.Execute(
		*execute,
		"mkdir",
		"-p",
		mountPointTmp,
	)

	/*
		##################################################
			UEFI
		##################################################
	*/

	// Determine if and where the UEFI device is currently mounted.
	mountpointsUEFI, err := utils.GetMountpoints(
		configData.UEFI.Disk,
		mountpointsJSON,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Unmount all mountpoints for the UEFI device
	err = utils.UnmountAll(*execute, mountpointsUEFI)
	if err != nil {
		log.Fatal(err)
	}

	// Zap the UEFI target device.
	log.Printf("Zapping %s.\n", configData.UEFI.Disk)
	utils.Execute(
		*execute,
		"sgdisk",
		"--zap-all",
		configData.UEFI.Disk,
	)

	// Run partprobe to update the partition table.
	log.Println("Running partprobe to update the partition table.")
	utils.ExecuteSilent(
		*execute,
		"partprobe",
	)

	// Prepare the UEFI disk.
	log.Printf("Preparing UEFI disk %s.\n", configData.UEFI.Disk)
	utils.Execute(
		*execute,
		"parted",
		"--script",
		"--fix",
		"--align",
		"optimal",
		configData.UEFI.Disk,
		"--",
		"mklabel",
		"gpt",
	)

	// Create the UEFI partition.
	log.Printf(
		"Creating UEFI partition on %s with label %s and size %s.\n",
		configData.UEFI.Disk,
		configData.UEFI.Label,
		configData.UEFI.Size,
	)
	utils.Execute(
		*execute,
		"parted",
		"--script",
		"--fix",
		"--align",
		"optimal",
		configData.UEFI.Disk,
		"--",
		"mkpart",
		configData.UEFI.Label,
		"fat32",
		"1MiB",
		configData.UEFI.Size,
	)

	// Set the ESP flag on the partition.
	log.Printf("Setting the ESP flag on %s.\n", configData.UEFI.Disk)
	utils.Execute(
		*execute,
		"parted",
		"--script",
		"--fix",
		"--align",
		"optimal",
		configData.UEFI.Disk,
		"--",
		"set",
		"1",
		"esp",
		"on",
	)

	// Create the NixOS configuration partition if it is enabled.
	if configData.NixOS.Config.Enabled {

		log.Printf("Creating NixOS config partition on %s.\n", configData.UEFI.Disk)
		utils.Execute(
			*execute,
			"parted",
			"--script",
			"--fix",
			"--align",
			"optimal",
			configData.UEFI.Disk,
			"--",
			"mkpart",
			"nixos-config",
			"xfs",
			configData.UEFI.Size,
			"100%",
		)

	} else {
		log.Println("Skipping NixOS config partition creation as it is disabled.")
	}

	// Sleep a few seconds to allow the partition table to update.
	if *execute {
		log.Println("Waiting...")
		time.Sleep(5 * time.Second)
	}

	// Format the UEFI partition.
	partitionNameUEFI := fmt.Sprintf("%s-part1", configData.UEFI.Disk)
	log.Printf("Formatting UEFI partition: %s\n", partitionNameUEFI)
	utils.Execute(
		*execute,
		"mkfs.vfat",
		"-n",
		"EFI",
		partitionNameUEFI,
	)

	// Format the NixOS config partition if it is enabled.
	var partitionNameNixOSConfig string
	if configData.NixOS.Config.Enabled {

		partitionNameNixOSConfig = fmt.Sprintf("%s-part2", configData.UEFI.Disk)
		log.Printf("Formatting NixOS config partition: %s\n", partitionNameNixOSConfig)
		utils.Execute(
			*execute,
			"mkfs.xfs",
			"-f",
			partitionNameNixOSConfig,
		)

	} else {
		log.Println("Skipping NixOS config partition formatting as it is disabled.")
	}

	/*
		##################################################
			ZFS Pool
		##################################################
	*/

	// Determine the name of the ZFS pool.
	zfsPoolName := configData.ZFS.Pool.Name

	// Destroy any existing ZFS pool using that name.
	log.Println("Destroying any existing ZFS pool on the disks.")
	utils.ExecuteSilent(
		*execute,
		"zpool",
		"destroy",
		"-f",
		zfsPoolName,
	)

	for _, zfsDisk := range configData.ZFS.Disks {

		// Determine if and where the ZFS device is currently mounted.
		mountpointsZFS, err := utils.GetMountpoints(zfsDisk, mountpointsJSON)
		if err != nil {
			log.Fatal(err)
		}

		// Unmount all mountpoints for the ZFS device
		err = utils.UnmountAll(*execute, mountpointsZFS)
		if err != nil {
			log.Fatal(err)
		}

		// Clear any current ZFS label on the disk.
		log.Printf("Clearing ZFS pool label on %s.\n", zfsDisk)
		utils.ExecuteSilent(
			*execute,
			"zpool",
			"labelclear",
			"-f",
			zfsDisk,
		)

		// Zap the ZFS Pool disks.
		log.Printf("Zapping %s.\n", zfsDisk)
		utils.Execute(
			*execute,
			"sgdisk",
			"--zap-all",
			zfsDisk,
		)
	}

	// Run partprobe to update the partition table.
	log.Println("Running partprobe to update the partition table.")
	utils.ExecuteSilent(
		*execute,
		"partprobe",
	)

	// Sleep a few seconds to allow the partition table to update.
	if *execute {
		log.Println("Waiting...")
		time.Sleep(5 * time.Second)
	}

	// ZFS pool arguments.
	zpoolArgs := []string{"create", "-f"}

	// If compression is enabled, add the compression option.
	if configData.ZFS.Pool.Compression {
		zpoolArgs = append(zpoolArgs, "-O", "compression=zstd-3")
	}

	// If encryption is enabled, add the encryption option.
	if configData.ZFS.Pool.Encryption {
		zpoolArgs = append(zpoolArgs, "-O", "encryption=aes-256-gcm")
		zpoolArgs = append(zpoolArgs, "-O", "keyformat=passphrase")
		zpoolArgs = append(zpoolArgs, "-O", "keylocation=prompt")
	}

	// Set additional file system properties using the '-O' flag.
	zpoolArgs = append(zpoolArgs, "-O", "acltype=posixacl")
	zpoolArgs = append(zpoolArgs, "-O", "atime=off")
	zpoolArgs = append(zpoolArgs, "-O", "relatime=off")
	zpoolArgs = append(zpoolArgs, "-O", "canmount=noauto")
	zpoolArgs = append(zpoolArgs, "-O", "logbias=throughput")
	zpoolArgs = append(zpoolArgs, "-O", "mountpoint=none")
	zpoolArgs = append(zpoolArgs, "-O", "normalization=formD")
	zpoolArgs = append(zpoolArgs, "-O", "primarycache=metadata")
	zpoolArgs = append(zpoolArgs, "-O", "recordsize=32K")
	zpoolArgs = append(zpoolArgs, "-O", "secondarycache=metadata")
	zpoolArgs = append(zpoolArgs, "-O", "sync=standard")
	zpoolArgs = append(zpoolArgs, "-O", "dnodesize=auto")
	zpoolArgs = append(zpoolArgs, "-O", "xattr=sa")

	// Set additional properties, features or compatibility using the '-o' flag.
	zpoolArgs = append(zpoolArgs, "-o", "autotrim=on")
	zpoolArgs = append(zpoolArgs, "-o", "ashift=12")

	// Set the temporary mount argument.
	zpoolArgs = append(zpoolArgs, "-R", mountPoint)

	// Add the pool name to the zpool arguments.
	zpoolArgs = append(zpoolArgs, zfsPoolName)

	// If there is more than one root disk, we need to mirror or stripe them.
	if len(configData.ZFS.Disks) > 1 {
		if configData.ZFS.Pool.Mirror {
			log.Println("Creating mirrored ZFS pool.")
			zpoolArgs = append(zpoolArgs, "mirror")
		} else if configData.ZFS.Pool.Stripe {
			log.Println("Creating striped ZFS pool.")
		}
	}

	// Append the root disks to the zpool arguments.
	zpoolArgs = append(zpoolArgs, configData.ZFS.Disks...)

	// Create the ZFS pool.
	log.Printf("Creating ZFS pool %s.\n", zfsPoolName)
	utils.Execute(
		*execute,
		"zpool",
		zpoolArgs...,
	)

	/*
		##################################################
			ZFS Datasets
		##################################################
	*/

	log.Println("Creating ZFS datasets.")

	// Create the root dataset.
	zfsDatasetPathRoot := path.Join(zfsPoolName, zfsDatasetRoot)
	log.Printf("Creating root dataset: %s\n", zfsDatasetPathRoot)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDatasetPathRoot,
	)

	// Create the boot dataset.
	zfsDatasetPathBoot := path.Join(zfsPoolName, zfsDatasetBoot)
	log.Printf("Creating boot dataset: %s\n", zfsDatasetPathBoot)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDatasetPathBoot,
	)

	// Create the home dataset.
	zfsDataSetPathHome := path.Join(zfsPoolName, zfsDatasetHome)
	log.Printf("Creating home dataset: %s\n", zfsDataSetPathHome)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathHome,
	)

	// Create the nix dataset.
	zfsDataSetPathNix := path.Join(zfsPoolName, zfsDatasetNixStore)
	log.Printf("Creating nix dataset: %s\n", zfsDataSetPathNix)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathNix,
	)

	// Create the swap dataset.
	zfsDataSetPathSwap := path.Join(zfsPoolName, zfsDatasetSwap)
	log.Printf("Creating swap dataset: %s\n", zfsDataSetPathSwap)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-V",
		configData.Swap.Size,
		zfsDataSetPathSwap,
	)

	// Create the tmp dataset.
	zfsDataSetPathTmp := path.Join(zfsPoolName, zfsDatasetTmp)
	log.Printf("Creating tmp dataset: %s\n", zfsDataSetPathTmp)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathTmp,
	)

	// Create the var dataset.
	zfsDataSetPathVar := path.Join(zfsPoolName, zfsDatasetVar)
	log.Printf("Creating var dataset: %s\n", zfsDataSetPathVar)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathVar,
	)

	// Create the lib dataset.
	zfsDataSetPathLib := path.Join(zfsPoolName, zfsDatasetLib)
	log.Printf("Creating lib dataset: %s\n", zfsDataSetPathLib)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathLib,
	)

	// Create the docker dataset.
	zfsDataSetPathDocker := path.Join(zfsPoolName, zfsDatasetDocker)
	log.Printf("Creating docker dataset: %s\n", zfsDataSetPathDocker)
	utils.Execute(
		*execute,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathDocker,
	)

	/*
		##################################################
			Mount directories
		##################################################
	*/

	log.Println("Mounting directories.")

	// Mount the root dataset.
	log.Printf("Mounting %s to %s.\n", zfsDatasetPathRoot, mountPoint)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDatasetPathRoot,
		mountPoint,
	)

	// Mount the boot dataset.
	log.Printf("Mounting %s to %s.\n", zfsDatasetPathBoot, mountPointBoot)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDatasetPathBoot,
		mountPointBoot,
	)

	// Mount the UEFI partition.
	log.Printf("Mounting %s to %s.\n", partitionNameUEFI, mountPointUEFI)
	utils.Execute(
		*execute,
		"mount",
		"-t",
		"vfat",
		"-o",
		"fmask=0077,dmask=0077,iocharset=iso8859-1,X-mount.mkdir",
		partitionNameUEFI,
		mountPointUEFI,
	)

	// Mount the NixOS config partition if it is enabled.
	if configData.NixOS.Config.Enabled {
		log.Printf("Mounting %s to %s.\n", partitionNameNixOSConfig, mountPointNixOSConfig)
		utils.Execute(
			*execute,
			"mount",
			"-o",
			"X-mount.mkdir",
			"-t",
			"xfs",
			partitionNameNixOSConfig,
			mountPointNixOSConfig,
		)
	} else {
		log.Println("Skipping NixOS config partition mounting as it is disabled.")
	}

	// Mount the home dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathHome, mountPointHome)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathHome,
		mountPointHome,
	)

	// Mount the nix dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathNix, mountPointNix)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathNix,
		mountPointNix,
	)

	// Mount the var dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathVar, mountPointVar)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathVar,
		mountPointVar,
	)

	// Mount the lib dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathLib, mountPointLib)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathLib,
		mountPointLib,
	)

	// Mount the docker dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathDocker, mountPointDocker)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathDocker,
		mountPointDocker,
	)

	// Mount the tmp dataset.
	log.Printf("Mounting %s to %s.\n", zfsDataSetPathTmp, mountPointTmp)
	utils.Execute(
		*execute,
		"mount",
		"-o",
		"X-mount.mkdir",
		"-t",
		"zfs",
		zfsDataSetPathTmp,
		mountPointTmp,
	)

	/*
		##################################################
			NixOS
		##################################################
	*/

	// Generate the NixOS configuration.
	log.Println("Generating NixOS configuration.")
	utils.Execute(
		*execute,
		"nixos-generate-config",
		"--root",
		mountPoint,
	)

	// Read the default NixOS configuration.
	if !*execute {
		log.Println("Dry run, skipping NixOS configuration modification...")
	} else {
		nixOSConfigPath := path.Join(mountPoint, "etc/nixos/configuration.nix")

		nixOSConfigDefault, err := os.ReadFile(nixOSConfigPath)
		validate.Error(err)

		// Replace the networking.hostId with the one from the config if provided.
		// Otherwise it will be generated
		var nixOSHostIDString string
		if configData.NixOS.HostID != "" {
			// Use the user provided host id.
			nixOSHostIDString = configData.NixOS.HostID
		} else {
			// Use the first 8 characters of the machine id.
			nixOSHostIDString = utils.ExecuteStdOut(
				*execute,
				"head",
				"-c",
				"8",
				"/etc/machine-id",
			)
		}
		regex := regexp.MustCompile("\n{\n")
		nixOSHostID := fmt.Sprintf("  networking.hostId = \"%s\";\n", nixOSHostIDString)
		nixOSConfigNew := regex.ReplaceAllString(string(nixOSConfigDefault), "\n{\n"+nixOSHostID+"\n")

		// Write the new NixOS configuration.
		err = os.WriteFile(nixOSConfigPath, []byte(nixOSConfigNew), os.ModePerm)
		validate.Error(err)
	}

	// Install NixOS.
	if *executeInstall {
		log.Println("Installing NixOS...")
		utils.Execute(
			*execute,
			"nixos-install",
			"--verbose",
			"--root",
			mountPoint,
			"--impure",
			"--flake",
			configData.NixOS.Flake,
		)
	} else {
		fmt.Println("")
		fmt.Println("You can now edit the NixOS configuration and install NixOS by running:")
		fmt.Println("")
		fmt.Println("export NIXPKGS_ALLOW_UNFREE=1")
		fmt.Printf("sudo -E nixos-install --verbose --root %s --impure --flake %s\n", mountPoint, configData.NixOS.Flake)
		fmt.Println("")
		fmt.Println("If needed, remember you can re-run the nixos-install command after making additional changes before rebooting.")
		fmt.Println("")
		if configData.NixOS.Config.Enabled {
			fmt.Printf("TIP: When using the NixOS config partition, it's a good idea to copy your flake locally to %s\n", mountPointNixOSConfig)
		}
	}

}
