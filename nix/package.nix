{
  lib,
  buildGoModule,
  makeWrapper,
  stdenv,
  zlib,
  uv,
  python313,
  version,
  commit,
}:

buildGoModule {
  pname = "mufradat";
  inherit version;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../embed.go
      ../LICENSE
      ../NOTICE
      ../assets
      ../cmd
      ../data
      ../internal
      ../python
    ];
  };

  vendorHash = "sha256-H6rdW+ElkiVxPPV0y9ygqjvbgLD66iAW+FHpAYk+rys=";

  subPackages = [ "cmd/mufradat" ];

  env.CGO_ENABLED = "0";

  ldflags = [
    "-s"
    "-w"
    "-X github.com/scuba-plaza/mufradat/internal/version.Version=${version}"
    "-X github.com/scuba-plaza/mufradat/internal/version.Commit=${commit}"
  ];

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/mufradat \
      --prefix PATH : ${lib.makeBinPath [ uv python313 ]} \
      --prefix LD_LIBRARY_PATH : ${lib.makeLibraryPath [ stdenv.cc.cc.lib zlib ]}

    mkdir -p $out/share/bash-completion/completions $out/share/zsh/site-functions $out/share/fish/vendor_completions.d
    $out/bin/mufradat completion bash > $out/share/bash-completion/completions/mufradat
    $out/bin/mufradat completion zsh > $out/share/zsh/site-functions/_mufradat
    $out/bin/mufradat completion fish > $out/share/fish/vendor_completions.d/mufradat.fish
  '';

  meta = {
    description = "Build Anki decks of common Modern Standard Arabic words with verified vowels and audio";
    homepage = "https://github.com/scuba-plaza/mufradat";
    license = lib.licenses.mit;
    mainProgram = "mufradat";
    platforms = [ "x86_64-linux" "aarch64-linux" ];
  };
}
