package installer

// Where nixos will be installed to.
const mountPoint = "/mnt/nixos"

// The names of the ZFS datasets.
const (
	zfsDatasetBoot       = "boot"
	zfsDatasetRoot       = "root"
	zfsDatasetHome       = "home"
	zfsDatasetNixStore   = "nix"
	zfsDatasetSwap       = "swap"
	zfsDatasetTmp        = "tmp"
	zfsDatasetVar        = "var"
	zfsDatasetLib        = "var/lib"
	zfsDatasetDocker     = "var/lib/docker"
	zfsDatasetContainers = "var/lib/containers"
	zfsDatasetIncus      = "var/lib/incus"
)
