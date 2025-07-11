{
  pkgs,
  name,
  version,
  ...
}:

pkgs.buildGoApplication {

  pname = name;
  inherit version;

  src = builtins.path {
    name = "source";
    path = ../src/.;
  };

  # NOTE: Generate this file with 'gomod2nix generate'
  modules = ../src/gomod2nix.toml;

}
