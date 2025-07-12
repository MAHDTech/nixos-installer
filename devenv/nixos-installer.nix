{
  pkgs,
  lib,
  name,
  version,
  ...
}:
let

  # Obtain the git commit hash using runCommand
  gitCommit = pkgs.lib.removeSuffix "\n" (
    pkgs.lib.readFile (
      pkgs.runCommand "git-commit"
        {
          nativeBuildInputs = [ pkgs.git ];
        }
        ''
          cd ${pkgs.lib.cleanSource ../.}
          if [ -d .git ];
          then
            git rev-parse HEAD | cut -c1-8 > $out
          else
            echo "unknown" > $out
          fi
        ''
    )
  );

  # Obtain the build date using runCommand
  buildDate = pkgs.lib.removeSuffix "\n" (
    pkgs.lib.readFile (
      pkgs.runCommand "build-date" { } ''
        date -u +%Y-%m-%dT%H:%M:%SZ > $out
      ''
    )
  );

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

      preBuild = ''
        export CGO_ENABLED=0
        export GOOS=${platform.goos}
        export GOARCH=${platform.goarch}
      '';

      nativeBuildInputs = with pkgs; [
        git
      ];

      ldflags = [
        "-s"
        "-w"
        "-X main.Version=${version}"
        "-X main.CommitSHA=${gitCommit}"
        "-X main.BuildDate=${buildDate}"
      ];

    };

  # Build all platforms
  allPlatforms = lib.mapAttrs (system: _: {
    normal = buildFor system "normal";
  }) platforms;
in
{

  # Export each platform build separately
  inherit (allPlatforms)
    # Linux
    aarch64-linux
    x86_64-linux

    # macOS
    aarch64-darwin
    x86_64-darwin

    ;

  # Export the default for the current platform
  default = allPlatforms.${pkgs.stdenv.hostPlatform.system};

  # Normal build.
  normal = buildFor pkgs.stdenv.hostPlatform.system "normal";
}
