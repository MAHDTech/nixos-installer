package installer

// Where nixos will be installed to.
const mountPoint = "/mnt/nixos"

// The names of the ZFS datasets.
const (
	zfsDatasetBoot            = "boot"
	zfsDatasetRoot            = "root"
	zfsDatasetHome            = "home"
	zfsDatasetNixStore        = "nix"
	zfsDatasetSwap            = "swap"
	zfsDatasetTmp             = "tmp"
	zfsDatasetVar             = "var"
	zfsDatasetLib             = "var/lib"
	zfsDatasetDocker          = "var/lib/docker"
	zfsDatasetContainers      = "var/lib/containers"
	zfsDatasetDRBD            = "var/lib/drbd"
	zfsDatasetIncus           = "var/lib/incus"
	zfsDatasetLinstorData     = "var/lib/linstor"
	zfsDatasetLinstorMetadata = "var/lib/linstor.d"
	zfsDatasetStoragePool     = "var/lib/storage-pools"
)

// The names of the tools that are required.
var requiredTools = []string{
	"zfs",
	"zpool",
	"sgdisk",
	"wipefs",
	"mount",
	"umount",
	"lsblk",
	"readlink",
	"partprobe",
	"udevadm",
	"dd",
	"chmod",
}
