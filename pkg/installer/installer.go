package installer

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"

	config "github.com/MAHDTech/nixos-installer/pkg/config"
	utils "github.com/MAHDTech/nixos-installer/pkg/utils"
	validate "github.com/MAHDTech/nixos-installer/pkg/validate"
)

const mountPoint = "/mnt/nixos"
const zfsPool = "zpool"
const zfsDatasetBoot = "boot"
const zfsDatasetRoot = "root"
const zfsDatasetHome = "home"
const zfsDatasetNixStore = "nix"
const zfsDatasetSwap = "swap"
const zfsDatasetTmp = "tmp"
const zfsDatasetVar = "var"

func usage() {

	fmt.Println(`
	Usage:
	
		nixos-installer -config <path/to/config.yaml> [-run]

	Options:
	
		-config string
			Path to the YAML configuration file (default "config.yaml")
		
		-run
			Add this flag to run the commands, otherwise it's a dry run

	Description:

		This tool installs NixOS based on the provided configuration file.

		By default, it runs in dry-run mode unless the '-run' flag is specified.
	`)

	flag.PrintDefaults()

}

func Run() {

	// If no flags are set, print the usage and exit.
	if len(os.Args) == 1 {
		usage()
		os.Exit(0)
	}

	// By default we run in dry run mode unless the 'run' flag is set
	// to avoid a user shooting themselves in the foot.
	dryRun := flag.Bool(
		"run",
		true,
		"Add this flag to run the commands, otherwise it's a dry run.",
	)

	// Determine the path to the configuration file.
	configFile := flag.String(
		"config",
		"config.yaml",
		"Path to the YAML configuration file",
	)

	// Parse the flags.
	flag.Parse()

	// Read the YAML configuration file and parse it into a Config struct.
	configData, err := config.ReadConfig(*configFile)
	validate.Error(err)

	// Validate the configuration.
	err = config.ValidateConfig(&configData)
	validate.Error(err)

	/*
		##################################################
			Mount directories
		##################################################
	*/

	// Create the directories where the temporary mount points will be created.
	log.Printf("Creating mount directory %s\n", mountPoint)
	utils.Execute(
		dryRun,
		"mkdir",
		"-p",
		mountPoint,
	)

	// Create mount point for 'boot'
	mountPointBoot := path.Join(mountPoint, "boot")
	log.Printf("Creating mount point for 'boot' at: %s\n", mountPointBoot)
	utils.Execute(
		dryRun,
		"mkdir",
		"-p",
		mountPointBoot,
	)

	// Create mount point for 'efi'
	mountPointUEFI := path.Join(mountPoint, "boot/efi")
	log.Printf("Creating mount point for 'efi' at: %s\n", mountPointUEFI)
	utils.Execute(
		dryRun,
		"mkdir",
		"-p",
		mountPointUEFI,
	)

	// Create mount point for 'home'
	mountPointHome := path.Join(mountPoint, "home")
	log.Printf("Creating mount point for 'home' at: %s\n", mountPointHome)
	utils.Execute(
		dryRun,
		"mkdir",
		"-p",
		mountPointHome,
	)

	// Create mount point for 'nix'
	mountPointNix := path.Join(mountPoint, "nix")
	log.Printf("Creating mount point for 'nix' at: %s\n", mountPointNix)
	utils.Execute(
		dryRun,
		"mkdir",
		"-p",
		mountPointNix,
	)

	// Create mount point for 'var'
	mountPointVar := path.Join(mountPoint, "var")
	log.Printf("Creating mount point for 'var' at: %s\n", mountPointVar)
	utils.Execute(
		dryRun,
		"mkdir",
		"-p",
		mountPointVar,
	)

	// Create mount point for 'tmp'
	mountPointTmp := path.Join(mountPoint, "tmp")
	log.Printf("Creating mount point for 'tmp' at: %s\n", mountPointTmp)
	utils.Execute(
		dryRun,
		"mkdir",
		"-p",
		mountPointTmp,
	)

	/*
		##################################################
			UEFI
		##################################################
	*/

	// Zap the UEFI target device.
	utils.Execute(
		dryRun,
		"sgdisk",
		"--zap-all",
		configData.UEFI.Disk,
	)

	// Prepare the UEFI disk.
	log.Printf("Preparing UEFI disk %s.\n", configData.UEFI.Disk)
	utils.Execute(
		dryRun,
		"parted",
		"--align",
		"optimal",
		configData.UEFI.Disk,
		"--",
		"mklabel",
		"gpt",
	)

	// Create the UEFI partition.
	log.Printf("Creating UEFI partition on %s with label %s and size %s.\n", configData.UEFI.Disk, configData.UEFI.Label, configData.UEFI.Size)
	utils.Execute(
		dryRun,
		"parted",
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
		dryRun,
		"parted",
		configData.UEFI.Disk,
		"--",
		"set",
		"1",
		"esp",
		"on",
	)

	// Format the UEFI partition.
	partitionNameUEFI := path.Join(configData.UEFI.Disk, "-part1")
	log.Printf("Formatting UEFI partition: %s\n", partitionNameUEFI)
	utils.Execute(
		dryRun,
		"mkfs.vfat",
		"-n",
		"EFI",
		partitionNameUEFI,
	)

	// Mount the UEFI partition.
	log.Printf("Mounting %s to %s.\n", partitionNameUEFI, mountPointUEFI)
	utils.Execute(
		dryRun,
		"mount",
		"-t",
		"vfat",
		"-o",
		"fmask=0077,dmask=0077,iocharset=iso8859-1,X-mount.mkdir",
		partitionNameUEFI,
		mountPointUEFI,
	)

	/*
		##################################################
			ZFS Pool
		##################################################
	*/

	// Zap the ZFS Pool disks.
	for _, zfsDisk := range configData.ZFS.Disks {
		utils.Execute(
			dryRun,
			"sgdisk",
			"--zap-all",
			zfsDisk,
		)
	}

	// ZFS pool arguments.
	zpoolArgs := []string{"create", "--force"}

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

	// Set the additional options.
	zpoolArgs = append(zpoolArgs, "-O", "acltype=posixacl")
	zpoolArgs = append(zpoolArgs, "-O", "canmount=off")
	zpoolArgs = append(zpoolArgs, "-O", "mountpoint=none")
	zpoolArgs = append(zpoolArgs, "-O", "xattr=sa")
	zpoolArgs = append(zpoolArgs, "-o", "ashift=12")
	zpoolArgs = append(zpoolArgs, "-o", "atime=off")
	zpoolArgs = append(zpoolArgs, "-o", "autotrim=on")
	zpoolArgs = append(zpoolArgs, "-o", "dnodesize=auto")
	zpoolArgs = append(zpoolArgs, "-o", "logbias=throughput")
	zpoolArgs = append(zpoolArgs, "-o", "normalization=formD")
	zpoolArgs = append(zpoolArgs, "-o", "primarycache=metadata")
	zpoolArgs = append(zpoolArgs, "-o", "recordsize=32K")
	zpoolArgs = append(zpoolArgs, "-o", "relatime=on")
	zpoolArgs = append(zpoolArgs, "-o", "secondarycache=metadata")
	zpoolArgs = append(zpoolArgs, "-o", "sync=standard")
	zpoolArgs = append(zpoolArgs, "-o", "zfs_prefetch_disable=1")

	// Set the temporary mount argument.
	zpoolArgs = append(zpoolArgs, "-R", mountPoint)

	// Add the pool name to the zpool arguments.
	zpoolArgs = append(zpoolArgs, zfsPool)

	// If there is more than one root disk, we need to mirror or stripe them.
	if len(configData.ZFS.Disks) > 1 {
		if configData.ZFS.Pool.Mirror {
			zpoolArgs = append(zpoolArgs, "mirror")
		} else if configData.ZFS.Pool.Stripe {
			zpoolArgs = append(zpoolArgs, "stripe")
		}
	}

	// Append the root disks to the zpool arguments.
	zpoolArgs = append(zpoolArgs, configData.ZFS.Disks...)

	// Create the ZFS pool.
	log.Printf("Creating ZFS pool %s.\n", zfsPool)
	utils.Execute(
		dryRun,
		"zpool",
		zpoolArgs...,
	)

	/*
		##################################################
			ZFS Datasets
		##################################################
	*/

	log.Println("Creating ZFS datasets.")

	// Create the boot dataset.
	zfsDatasetPathBoot := path.Join(zfsPool, zfsDatasetBoot)
	log.Printf("Creating boot dataset: %s\n", zfsDatasetPathBoot)
	utils.Execute(
		dryRun,
		"zfs",
		"create",
		"-o",
		"canmount=noauto",
		"-o",
		"mountpoint=legacy",
		zfsDatasetPathBoot,
	)

	// Create the root dataset.
	zfsDatasetPathRoot := path.Join(zfsPool, zfsDatasetRoot)
	log.Printf("Creating root dataset: %s\n", zfsDatasetPathRoot)
	utils.Execute(
		dryRun,
		"zfs",
		"create",
		"-o",
		"canmount=noauto",
		"-o",
		"mountpoint=legacy",
		zfsDatasetPathRoot,
	)

	// Create the home dataset.
	zfsDataSetPathHome := path.Join(zfsPool, zfsDatasetHome)
	log.Printf("Creating home dataset: %s\n", zfsDataSetPathHome)
	utils.Execute(
		dryRun,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathHome,
	)

	// Create the nix dataset.
	zfsDataSetPathNix := path.Join(zfsPool, zfsDatasetNixStore)
	log.Printf("Creating nix dataset: %s\n", zfsDataSetPathNix)
	utils.Execute(
		dryRun,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathNix,
	)

	// Create the swap dataset.
	zfsDataSetPathSwap := path.Join(zfsPool, zfsDatasetSwap)
	log.Printf("Creating swap dataset: %s\n", zfsDataSetPathSwap)
	utils.Execute(
		dryRun,
		"zfs",
		"create",
		"-V",
		configData.Swap.Size,
		zfsDataSetPathSwap,
	)

	// Create the tmp dataset.
	zfsDataSetPathTmp := path.Join(zfsPool, zfsDatasetTmp)
	log.Printf("Creating tmp dataset: %s\n", zfsDataSetPathTmp)
	utils.Execute(
		dryRun,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathTmp,
	)

	// Create the var dataset.
	zfsDataSetPathVar := path.Join(zfsPool, zfsDatasetVar)
	log.Printf("Creating var dataset: %s\n", zfsDataSetPathVar)
	utils.Execute(
		dryRun,
		"zfs",
		"create",
		"-o",
		"mountpoint=legacy",
		zfsDataSetPathVar,
	)

	/*
		##################################################
			NixOS
		##################################################
	*/

	// Generate the NixOS configuration.
	log.Println("Generating NixOS configuration.")
	utils.Execute(
		dryRun,
		"nixos-generate-config",
		"--root",
		mountPoint,
	)

	// Read the default NixOS configuration.
	if *dryRun {
		log.Println("Dry run, skipping NixOS configuration...")
	} else {
		nixOSConfigPath := path.Join(mountPoint, "etc/nixos/configuration.nix")

		nixOSConfigDefault, err := os.ReadFile(nixOSConfigPath)
		validate.Error(err)

		// Replace the networking.hostId with the one from the config.
		regex := regexp.MustCompile("\n{\n")
		nixOSHostId := fmt.Sprintf("  networking.hostId = \"%s\";\n", configData.HostID)
		nixOSConfigNew := regex.ReplaceAllString(string(nixOSConfigDefault), "\n{\n"+nixOSHostId+"\n")

		// Write the new NixOS configuration.
		err = os.WriteFile(nixOSConfigPath, []byte(nixOSConfigNew), os.ModePerm)
		validate.Error(err)

		// Install NixOS.
		log.Println("Installing NixOS.")
		utils.Execute(
			dryRun,
			"nixos-install",
			"--root",
			mountPoint,
			"--flake",
			configData.Flake,
		)

	}

}
