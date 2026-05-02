# Usage notes

`Xray-Link-Json` accepts one input argument: inline JSON, an inline share link, a file path, or `-` for stdin.

## Diagnostics

The conversion result is written to stdout. Converter diagnostics and Xray warnings are written to stderr so stdout stays usable in pipelines.

Example:

```bash
cat tmp/test_1 | Xray-Link-Json - > converted.json 2> conversion.log
```

## Empty outbound tags

When share links are converted to Xray JSON, any outbound with an empty `"tag": ""` value gets a random hyphenated two-word tag such as `"blue-bat"`. Words are selected from `/usr/share/dict/` when available, with a small built-in fallback word list if dictionaries cannot be read.

