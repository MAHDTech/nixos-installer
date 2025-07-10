{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils";
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
            ];
            GOPROXY = "off";
            GOSUMDB = "off";
            inherit (pkgs) go;
          };
        };
    in
    with utils.lib;
    eachSystem defaultSystems out;
}
