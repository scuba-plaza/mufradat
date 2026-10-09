{
  description = "Build Anki decks of common Modern Standard Arabic words";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      version = if self ? shortRev then "git-${self.shortRev}" else "dev";
      commit = self.rev or self.dirtyRev or "";
    in
    {
      overlays.default = final: _prev: {
        mufradat = final.callPackage ./nix/package.nix { inherit version commit; };
      };

      packages = forAll (pkgs: rec {
        mufradat = pkgs.callPackage ./nix/package.nix { inherit version commit; };
        default = mufradat;
      });

      apps = forAll (pkgs: {
        default = {
          type = "app";
          program = "${self.packages.${pkgs.system}.default}/bin/mufradat";
        };
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            golangci-lint
            goreleaser
            just
            uv
            python313
            nodejs
            typescript
          ];
          LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath [ pkgs.stdenv.cc.cc.lib pkgs.zlib ];
        };
      });

      checks = forAll (pkgs: {
        build = self.packages.${pkgs.system}.default;
      });
    };
}
