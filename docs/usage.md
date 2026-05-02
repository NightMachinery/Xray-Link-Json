# Usage notes

`Xray-Link-Json` accepts one input argument: inline JSON, an inline share link, a file path, or `-` for stdin.

## Outbound-only output

Use `--outbound-only` when converting share links to Xray JSON and you only want the outbound entries:

```bash
cat tmp/test_1 | Xray-Link-Json --outbound-only -
```

If the input is already an Xray JSON config, `--outbound-only` extracts its existing `outbounds` entries instead of converting the config to share links.

This mode prints only the comma-separated outbound JSON objects. It intentionally omits the surrounding `[` and `]` so the output can be pasted directly inside an existing config section such as:

```json
{
  "outbounds": [
    /* paste --outbound-only output here */
  ]
}
```

Any converter diagnostics or Xray warnings are written to stderr, not stdout, so stdout stays usable as a JSON snippet.
