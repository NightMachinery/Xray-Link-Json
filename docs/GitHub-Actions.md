# Tag to Trigger Building Release Artifacts

This command creates an **annotated Git tag** named `v0.2.0`:

```bash
git tag -a v0.2.0 -m "v0.2.0"
```

Breakdown:

`git tag`
Creates, lists, or manages Git tags.

`-a`
Means **annotated tag**. Git creates a full tag object with metadata, including:

* tag name
* tag message
* tagger name/email
* date
* the commit it points to

`v0.2.0`
The tag name. This usually means “version 0.2.0”.

`-m "v0.2.0"`
Adds the tag message. Here the message is also `v0.2.0`.

By default, this tags your **current commit**, meaning whatever commit `HEAD` points to.

After running it, you usually push the tag with:

```bash
git push origin v0.2.0
```

Or push all local tags:

```bash
git push origin --tags
```

A more descriptive version might be:

```bash
git tag -a v0.2.0 -m "Release version 0.2.0"
```

Annotated tags are typically preferred for releases because they store extra metadata.

# Watch Progress

```
gh run watch --repo NightMachinery/Xray-Link-Json --exit-status
```

