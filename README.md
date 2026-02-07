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

Behavior:
- If input starts with `{`, it is treated as Xray JSON and converted to share links.
- Otherwise, it is treated as a share link and converted to Xray JSON.

### Examples

Convert link -> JSON:

```bash
Xray-Link-Json 'vless://123456789@example.com:443?security=tls&sni=sni.example.com&type=ws&host=host.example.com&path=%2F#sample-vless'
```

Convert JSON -> link:

```bash
Xray-Link-Json '{"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"example.com","port":443,"users":[{"id":"123456789","encryption":"none"}]}]}}]}'
```

From a file:

```bash
Xray-Link-Json "$(jq -c . ./client.json)"
```

## Notes

- Output is printed as decoded JSON text.
- The tool currently logs the raw input line before conversion.

## License

MIT
