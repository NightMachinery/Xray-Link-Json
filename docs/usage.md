# Usage notes

`Xray-Link-Json` accepts one input argument: inline JSON, an inline share link, a file path, or `-` for stdin.

Print build metadata with either form:

```bash
Xray-Link-Json --version
Xray-Link-Json version
```

The output includes the application version, commit, build date, and bundled
Xray version.

Bare proxy URLs are accepted directly:

```bash
Xray-Link-Json 'socks5://127.0.0.1:10050'
Xray-Link-Json 'socks5://user:pass@example.com:1080#proxy-tag'
Xray-Link-Json 'http://proxyuser:password@example.com:2060'
```

For `http://` inputs, only bare proxy URLs with no path or query string are
handled as proxy settings. Other HTTP URLs are left for the upstream converter.

## Release artifacts

GitHub Releases are built from version tags such as `v1.2.3`.

Release builds can stamp version metadata with linker flags:

```bash
go build -ldflags "\
-X main.version=${TAG} \
-X main.commit=${COMMIT} \
-X main.date=${DATE}"
```

Prebuilt archives use this naming pattern:

```text
Xray-Link-Json_<tag>_<os>_<arch>.tar.gz
Xray-Link-Json_<tag>_<os>_<arch>.zip
```

Windows builds use `.zip`; Linux, macOS, and Android builds use `.tar.gz`.
The Android arm64 artifact is intended for Termux-style usage on modern Android
devices. Each release also includes `checksums.txt` with SHA-256 hashes.

### Linux ARM vs Android ARM

`linux_arm64` and `android_arm64` both target 64-bit ARM CPUs, but they are not
the same operating-system target:

- `linux_arm64` is for standard GNU/Linux or musl Linux distributions on ARM64,
  such as Debian, Ubuntu, Alpine, or Raspberry Pi OS.
- `android_arm64` is for Android userlands, especially Termux on ARM64 phones,
  tablets, and devices.

Android uses its own OS ABI, libc, filesystem conventions, and syscall surface.
Use the Android artifact on Termux even if the device reports an ARM64 CPU. Use
the Linux artifact only for a normal Linux distribution.

## Diagnostics

The conversion result is written to stdout. Converter diagnostics and Xray warnings are written to stderr so stdout stays usable in pipelines.

Example:

```bash
cat tmp/test_1 | Xray-Link-Json - > converted.json 2> conversion.log
```

## Empty outbound tags

When share links are converted to Xray JSON, any outbound with an empty `"tag": ""` value gets a random hyphenated two-word tag such as `"blue-bat"`. Words are selected from `/usr/share/dict/` when available, with a small built-in fallback word list if dictionaries cannot be read.
