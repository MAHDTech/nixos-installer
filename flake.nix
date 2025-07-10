{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    sysutil.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
    }:
    let
      out =
        system:
        let
          pkgs = nixpkgs.legacyPackages."${system}";

          # Extract repository and ref information
          sourceInfo = self.sourceInfo or { };
          # Extract repo from URL like "github:MAHDTech/nixos-installer" or "github:MAHDTech/nixos-installer/branch"
          repoUrl = sourceInfo.url or "github:MAHDTech/nixos-installer";
          # Parse the GitHub repo from the URL
          githubRepo =
            let
              urlParts = builtins.split "/" repoUrl;
              # Handle formats like "github:MAHDTech/nixos-installer" or "github:MAHDTech/nixos-installer/branch"
              repoMatch = builtins.match "github:([^/]+)/([^/]+).*" repoUrl;
            in
            if repoMatch != null then
              "${builtins.elemAt repoMatch 0}/${builtins.elemAt repoMatch 1}"
            else
              "MAHDTech/nixos-installer"; # fallback

          # Use ref if available, otherwise fall back to trunk
          gitRef = self.ref or self.rev or "trunk";

        in
        {
          defaultPackage = pkgs.buildGoModule {
            pname = "nixos-installer";
            version = "0.1.0";
            src = ./.;
            vendorHash = null;
            env.CGO_ENABLED = "0";
            ldflags = [
              "-s"
              "-w"
              "-X github.com/MAHDTech/nixos-installer/pkg/config.GitHubRepo=${githubRepo}"
              "-X github.com/MAHDTech/nixos-installer/pkg/config.GitRef=${gitRef}"
            ];
            GOPROXY = "off";
            GOSUMDB = "off";
            inherit (pkgs) go;
          };
        };
    in
    with sysutil.lib;
    eachSystem defaultSystems out;
}
