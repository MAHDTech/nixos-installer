# TODO

Ok, here is the current TODO list were working on.

## Release Process

The release process will be as follows;

- Each time a PR is merged into trunk, a new release will be created if the criteria is met.
- Each release will have a git tag based on semantic versioning.
- If an existing tag with the same version exists, no release is made.
- The semantic version will need to be updated in the devenv.nix file
- A pre-commit hook should check the current semantic version defined vs the last commit on trunk
- The pre-commit hook should fail is the version was not updated by the user.
- Determine how to read the version from devenv.nix
  - Perhaps use this to obtain it.
  - devenv build outputs.nixos-installer
  - /nix/store/y0pydyq2xidar665bww3i3883g958nxp-nixos-installer-1.0.0
  - APP_PATH=$(devenv build outputs.nixos-installer)
  - APP_VER=$(echo $APP_PATH | cut -d "-" -f 4)
  - echo $APP_VER
  - 1.0.0

## Phase 1: Fix Current Issues

- [x] Review and update the go ci workflow to have binary releases on merges into trunk
- [x] Create install script for one-shot usage named scripts/yolo.sh
- [x] Update README.md with new usage instructions and remove old.

## Phase 2: Enhance Functionality

- [x] Add multi-architecture builds (amd64, arm64) into devenv
- [ ] Implement automatic releases on tag creation using semantic versioning
- [ ] Add checksum verification for downloads in the shell script, a checksums.txt file will need to be added into the release

## Phase 3: Documentation & Testing

- [x] Update all documentation with new workflow
- [ ] Test all scenarios (local config, remote config, different architectures)
