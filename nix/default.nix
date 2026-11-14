{ lib, stdenv, nodejs, pnpm, fetchPnpmDeps, pnpmConfigHook, makeWrapper }:

stdenv.mkDerivation (finalAttrs: {
  pname = "mufradat";
  version = "1.0.0";

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../bin
      ../data
      ../packages
      ../public
      ../src
      ../next.config.mjs
      ../package.json
      ../pnpm-lock.yaml
      ../pnpm-workspace.yaml
      ../postcss.config.js
      ../tailwind.config.js
      ../tsconfig.json
    ];
  };

  pnpmDeps = fetchPnpmDeps {
    inherit (finalAttrs) pname version src;
    inherit pnpm;
    fetcherVersion = 5;
    hash = "sha256-xgTHMx1lIDeAUQ7CCj0+PhTlimhLjCP1n/UrQQDDEng=";
  };

  nativeBuildInputs = [ nodejs pnpm pnpmConfigHook makeWrapper ];

  buildPhase = ''
    runHook preBuild
    pnpm build
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    mkdir -p $out/lib/mufradat $out/bin
    cp -r .next bin data packages public node_modules package.json next.config.mjs $out/lib/mufradat/
    makeWrapper ${nodejs}/bin/node $out/bin/mufradat \
      --chdir $out/lib/mufradat \
      --add-flags node_modules/next/dist/bin/next \
      --add-flags "start -H 127.0.0.1"
    runHook postInstall
  '';

  meta = {
    description = "Local web application for building an Arabic vocabulary deck for Anki";
    mainProgram = "mufradat";
  };
})
