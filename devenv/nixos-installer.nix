{
  pkgs,
  lib,
  name,
  version,
  ...
}:
let

  # Supported platforms
  platforms = {
    "x86_64-linux" = {
      goos = "linux";
      goarch = "amd64";
    };
    "aarch64-linux" = {
      goos = "linux";
      goarch = "arm64";
    };
    "x86_64-darwin" = {
      goos = "darwin";
      goarch = "amd64";
    };
    "aarch64-darwin" = {
      goos = "darwin";
      goarch = "arm64";
    };
  };

  # Build for a specific platform
  buildFor =
    system: type:
    let
      platform = platforms.${system};
    in
    pkgs.buildGoApplication rec {

      pname = "${name}_${version}_${platform.goos}-${platform.goarch}";
      inherit version;

      src = builtins.path {
        name = "source";
        path = ../src/.;
      };

      # NOTE: Generate this file with the following command:
      # pushd src ; gomod2nix generate ; popd
      modules = ../src/gomod2nix.toml;

    };

  # Build all platforms
  allPlatforms = lib.mapAttrs (system: _: {
    normal = buildFor system "normal";
  }) platforms;
in
{

  # Export each platform build separately
  inherit (allPlatforms)
    aarch64-darwin
    aarch64-linux
    x86_64-darwin
    x86_64-linux
    ;

  # Export the default for the current platform
  default = allPlatforms.${pkgs.stdenv.hostPlatform.system};

  # Normal build.
  normal = buildFor pkgs.stdenv.hostPlatform.system "normal";
}
