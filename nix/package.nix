{
  lib,
  buildGoModule,
  go_1_25,
  squad-aegis-web,
  sourceInfo ? null,
}:

let
  # Prefer the flake's git revision; fall back to a stable string for
  # `nix-build` / non-flake consumers.
  version =
    if sourceInfo == null then
      "0-unstable"
    else
      "0-unstable-" + (sourceInfo.shortRev or sourceInfo.dirtyShortRev or "dirty");
in
buildGoModule.override { go = go_1_25; } {
  pname = "squad-aegis";
  inherit version;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../cmd
      ../internal
      ../pkg
      ../third_party
      # Package dir + build-tagged embed file; the static assets are injected
      # below so the `embed` directive resolves at build time.
      ../web/web.go
      ../web/web_embed.go
    ];
  };

  vendorHash = "sha256-qZX3WKO513XVcgdRZHbGVqwGIm0lS0vhh7POX3HtR5k=";

  # The vendor derivation only needs go.mod/go.sum/sources — strip the
  # frontend-embedding preBuild so it doesn't depend on (or wait for) the
  # web build, and stays independently cacheable.
  overrideModAttrs = _: { preBuild = ""; };

  subPackages = [ "cmd/server" ];

  # Pure-Go build (clickhouse-go, pgx, discordgo … need no cgo): static binary.
  env.CGO_ENABLED = 0;

  tags = [ "embed" ];

  ldflags = [
    "-s"
    "-w"
    "-X go.codycody31.dev/squad-aegis/internal/version.Version=${version}"
  ];

  # Place the pre-built frontend where `//go:embed all:.output/public` expects
  # it, relative to the `web` package directory.
  preBuild = ''
    mkdir -p web/.output/public
    cp -r ${squad-aegis-web}/. web/.output/public/
  '';

  # Tests require live PostgreSQL/ClickHouse/Valkey; not run in the sandbox.
  doCheck = false;

  meta = {
    description = "Control panel for Squad game server administration";
    homepage = "https://github.com/Codycody31/squad-aegis";
    license = lib.licenses.gpl3Only;
    mainProgram = "squad-aegis";
    platforms = lib.platforms.unix;
  };
}
