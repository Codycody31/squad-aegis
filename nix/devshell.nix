{
  mkShell,
  go_1_25,
  gopls,
  gotools,
  go-tools,
  golangci-lint,
  gofumpt,
  delve,
  protobuf,
  protoc-gen-go,
  protoc-gen-go-grpc,
  nodejs_22,
  pnpm_9,
  postgresql,
}:

mkShell {
  name = "squad-aegis-dev";

  packages = [
    # Backend
    go_1_25
    gopls
    gotools
    go-tools
    golangci-lint
    gofumpt
    delve

    # Native plugin / connector gRPC stub generation (`make generate-proto`)
    protobuf
    protoc-gen-go
    protoc-gen-go-grpc

    # Frontend
    nodejs_22
    pnpm_9

    # `psql` for poking the dev database from docker-compose
    postgresql
  ];

  env.CGO_ENABLED = "0";

  shellHook = ''
    echo "squad-aegis dev shell — go $(go version | cut -d' ' -f3), node $(node --version), pnpm $(pnpm --version)"
    echo "  make build        build web UI + server"
    echo "  make generate     run code + proto generation"
    echo "  docker-compose -f docker-compose.dev.yml up   start backing services"
  '';
}
