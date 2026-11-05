{
  description = "Local web application for building an Arabic vocabulary deck for Anki";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs =
    { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      overlays.default = final: _prev: {
        mufradat = final.callPackage ./nix/default.nix { };
      };

      packages = forAll (pkgs: rec {
        mufradat = pkgs.callPackage ./nix/default.nix { };
        default = mufradat;
      });
    };
}
