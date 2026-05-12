# Xray-Link-Json

CLI wrapper around `github.com/xtls/libxray` that converts between:
- Xray JSON config
- share links (`vless://`, `vmess://`, etc.)
- bare proxy URLs (`socks5://host:port`, `http://user:pass@host:port`)

## Install

```bash
go install github.com/NightMachinery/Xray-Link-Json@latest
```

Requirements:
- Go 1.26.3+ (Go may auto-download a compatible toolchain)

## Release binaries

Versioned GitHub Releases provide prebuilt archives for Linux, macOS, Windows,
and Android arm64. Archives are named like
`Xray-Link-Json_v1.2.3_linux_amd64.tar.gz` or
`Xray-Link-Json_v1.2.3_windows_amd64.zip`, and each release includes a
`checksums.txt` file with SHA-256 hashes.

Use the `linux_arm64` build on normal 64-bit ARM Linux distributions, such as
Debian, Ubuntu, Alpine, or Raspberry Pi OS. Use the `android_arm64` build on
Android environments such as Termux. Even when the CPU architecture is the same,
Android is a different OS target with a different runtime/libc and syscall
surface, so Linux ARM binaries should not be treated as Android binaries.

## Usage

```bash
Xray-Link-Json '<input>'
Xray-Link-Json --version
```

Supported input forms:
- inline JSON
- inline share link
- path to a file containing JSON or share link
- `-` to read from stdin

Behavior:
- JSON input may include `// ...` and `/* ... */` comments.
- If resolved input is JSON (after trimming whitespace/comments), it is treated as Xray JSON and converted to share links.
- Otherwise, it is treated as share-link input and converted to Xray JSON.

### Examples

Convert link -> JSON:

```bash
Xray-Link-Json 'vless://123456789@example.com:443?security=tls&sni=sni.example.com&type=ws&host=host.example.com&path=%2F#sample-vless'
```

Convert a bare SOCKS or HTTP proxy URL -> JSON:

```bash
Xray-Link-Json 'socks5://127.0.0.1:10050'
Xray-Link-Json 'http://proxyuser:password@example.com:2060'
```

Convert JSON -> link:

```bash
Xray-Link-Json '{"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"example.com","port":443,"users":[{"id":"123456789","encryption":"none"}]}]}}]}'
```

From a file path:

```bash
Xray-Link-Json ./client.json
```

From stdin:

```bash
jq -c . ./client.json | Xray-Link-Json -
```

Diagnostics and Xray warnings are written to stderr so stdout contains only the converted data.

When converting share links to JSON, empty outbound `tag` fields are filled with random hyphenated two-word tags from the system dictionary.

`Xray-Link-Json --version` and `Xray-Link-Json version` print the build version,
commit, build date, and bundled Xray version.

## Notes

- Output is printed as decoded JSON text.
- Additional usage details are in [`docs/usage.md`](docs/usage.md).

## License

MIT
