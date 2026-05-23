# Releases

Releases are created with the GitHub Actions **Release** workflow.

## Before Release

1. Update `VERSION`.
2. Add a matching entry to `CHANGELOG.md`.
3. Run `make qa`.
4. Commit the version and changelog changes.
5. Trigger the **Release** workflow manually with the same version value.

The workflow validates:

- the manual input or tag version matches `VERSION`
- `CHANGELOG.md` contains a heading for that version

The release job uses `whilesmart/workflows/go/release@main` to build multi-platform binaries.

## Version Format

`VERSION` stores the plain semantic version:

```text
0.1.0
```

Git tags use the `v` prefix:

```text
v0.1.0
```
