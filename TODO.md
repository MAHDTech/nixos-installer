# TODO

Ok, here is the current TODO list were working on.

## Release Process

The release process will following these specifications:

- On PRs

  - Go build and test will run
  - devenv build and test will run

- On merges into trunk

  - The latest git tag is obtained
  - The version number is captured from the devenv.nix
  - If a tag for the versions already exists, no action is taken
  - If no tag exists matching the version a new tag is created
  - If a tag is created an output is set to trigger release
  - When a release is created, release notes are generated using githubs built in release notes
  - the go binaries are checksummed and checksums.txt uploaded into the release using gh cli
  - the go binaries are uploaded into the release using the gh cli

- The version can be obtained as follows from devenv.nix
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
- [x] Add version flag support to the Go program
- [x] Update devenv/nixos-installer.nix to properly cross-compile with GOOS/GOARCH
- [x] Add build-time version injection via ldflags

## Phase 2: Enhance Functionality

- [x] Add multi-architecture builds (amd64, arm64) into devenv
- [ ] Improve the CI with automatic releases on tag creation using semantic versioning from devenv.nix
- [ ] Add checksum verification for downloads in the shell script, a checksums.txt file will need to be added into the release
- [ ] Add pre-commit hook to check version bump requirements
- [ ] Test the version flag functionality in CI
- [ ] Add version information to the help output

## Phase 3: Documentation & Testing

- [x] Update all documentation with new workflow
- [ ] Test all scenarios (local config, remote config, different architectures)
- [ ] Add integration tests for version flag
- [ ] Document the release process for maintainers

## Phase 4: Security & Quality

- [ ] Add security scanning (Trivy) to CI workflows
- [ ] Add dependency vulnerability scanning
- [ ] Implement proper error handling for version extraction
- [ ] Add unit tests for version parsing logic
