# Xray-Link-Json

CLI wrapper around `github.com/xtls/libxray` that converts between:
- Xray JSON config
- share links (`vless://`, `vmess://`, etc.)

## Install

```bash
go install github.com/NightMachinery/Xray-Link-Json@latest
```

Requirements:
- Go 1.23.5+ (Go may auto-download a compatible toolchain)

## Usage

```bash
Xray-Link-Json '<input>'
```

Supported input forms:
- inline JSON
- inline share link
- path to a file containing JSON or share link
- `-` to read from stdin

Behavior:
- If resolved input starts with `{` or `[`, it is treated as Xray JSON and converted to share links.
- Otherwise, it is treated as share-link input and converted to Xray JSON.

### Examples

Convert link -> JSON:

```bash
Xray-Link-Json 'vless://123456789@example.com:443?security=tls&sni=sni.example.com&type=ws&host=host.example.com&path=%2F#sample-vless'
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

## Notes

- Output is printed as decoded JSON text.

## License

MIT
