# Xray/libxray upgrade notes

This project depends on `github.com/xtls/libxray`, which bundles Xray-core.
When bumping it, update both the dependency and conversion smoke tests.

## Choose a version

Upstream libxray currently publishes calendar tags such as `v26.3.27`, but the
module path in `go.mod` is still `github.com/xtls/libxray`. Go rejects those
tags because major versions above v1 require a matching `/vN` module path.

Use a pseudo-version from the desired commit instead:

```bash
git ls-remote https://github.com/XTLS/libXray.git HEAD
go list -m -json github.com/xtls/libxray@<commit>
go get github.com/xtls/libxray@<commit-or-pseudo-version>
go mod tidy
```

## Check API changes

Inspect the exported API before editing code:

```bash
go doc github.com/xtls/libxray
go doc github.com/xtls/libxray/share
```

The old v1 API exposed file-path functions on the root package:

```text
ParseShareText(inputPath, outputPath)
ConvertXrayJsonToShareText(inputPath, outputPath)
```

Current libxray exposes direct conversion helpers in the `share` subpackage:

```text
share.ConvertShareLinksToXrayJson(string)
share.ConvertXrayJsonToShareLinks([]byte)
```

Prefer the direct `share` package API for this CLI. The root package wrappers
are mobile-oriented and use base64 request/response envelopes.

## Required checks

Run local tests:

```bash
go test ./...
```

It is also useful to test upstream's share package from the module cache:

```bash
go test ./share
```

Full upstream libxray tests may fail in the module cache if they try to write
fixture files into the read-only module directory. A passing `./share` test is
the important signal for this converter.

## Smoke tests

Use synthetic data only. Do not paste private config paths, hostnames, UUIDs,
tags, or transport paths into tests or docs.

Link to JSON:

```bash
go run . 'vless://00000000-0000-0000-0000-000000000000@203.0.113.10:443?type=xhttp&encryption=none&path=%2Fxhttp-test&host=edge.example.com&mode=auto&security=tls&sni=edge.example.com'
```

JSON to link:

```bash
go run . '{"outbounds":[{"protocol":"vless","settings":{"address":"203.0.113.10","port":443,"id":"00000000-0000-0000-0000-000000000000","encryption":"none"},"streamSettings":{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"edge.example.com"},"xhttpSettings":{"path":"/xhttp-test","host":"edge.example.com","mode":"auto"}}}]}'
```

Confirm the output preserves `type=xhttp`, `path`, `host`, `mode`, and TLS SNI.
