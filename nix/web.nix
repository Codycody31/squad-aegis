{
  lib,
  stdenvNoCC,
  nodejs_22,
  pnpm_9,
  fetchPnpmDeps,
}:

# Builds the Nuxt 3 frontend into a static site. The Go server embeds the
# contents of this derivation at `web/.output/public` via the `embed` build tag.
stdenvNoCC.mkDerivation (finalAttrs: {
  pname = "squad-aegis-web";
  version = "0-unstable";

  src = lib.fileset.toSource {
    root = ../web;
    # Only the inputs needed to produce the static site; keeps the
    # fixed-output dependency fetch and the build itself cache-friendly.
    fileset = lib.fileset.unions [
      ../web/package.json
      ../web/pnpm-lock.yaml
      (lib.fileset.fileFilter (f: f.name != ".nuxt" && f.name != ".output") ../web)
    ];
  };

  pnpmDeps = fetchPnpmDeps {
    inherit (finalAttrs) pname version src;
    pnpm = pnpm_9;
    fetcherVersion = 3;
    hash = "sha256-H29vuICdm+IPuupxa7KjZFrHtB44Sv5s+ZgZm2FJHm8=";
  };

  nativeBuildInputs = [
    nodejs_22
    pnpm_9.configHook
  ];

  env = {
    # Nuxt/unjs phone home and CI prompts off — keeps the build hermetic.
    NUXT_TELEMETRY_DISABLED = "1";
    DO_NOT_TRACK = "1";
    CI = "true";
  };

  buildPhase = ''
    runHook preBuild
    pnpm run generate
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    cp -r .output/public "$out"
    runHook postInstall
  '';

  meta = {
    description = "Static frontend for Squad Aegis";
    homepage = "https://github.com/Codycody31/squad-aegis";
    license = lib.licenses.gpl3Only;
    platforms = lib.platforms.all;
  };
})
