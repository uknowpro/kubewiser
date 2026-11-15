# Releases

End users do not need Go, Helm, or kubectl for the currently implemented offline CLI and MCP tool. Build maintainers need Go (pinned in `.go-version`), Git, Bash, tar and a SHA-256 utility.

## Platforms

| OS | Architecture | Archive suffix |
| --- | --- | --- |
| Linux | Intel/AMD 64-bit | linux_amd64 |
| Linux | ARM64 | linux_arm64 |
| macOS | Intel | darwin_amd64 |
| macOS | Apple Silicon | darwin_arm64 |

Builds use `CGO_ENABLED=0`; Linux binaries do not require a system libc. OS compatibility still follows the pinned Go toolchain's supported OS versions. macOS binaries are not Developer ID signed or notarized. Cross-compilation alone is not a native runtime test. CI is configured to run tests on all four platform/architecture runners.

## Local preparation

```sh
go test -race ./...
go vet ./...
bash scripts/release.sh v0.1.0-alpha.1
```

Outputs are under `dist/v0.1.0-alpha.1/`: four tar.gz archives containing the executable, LICENSE and README, plus SHA-256 checksums and build metadata. Existing output directories are rejected. Build from a clean checkout for a release; archives embed the version and Git commit. Archives are not claimed to be byte-for-byte reproducible.

## GitHub release

Commit and push the release preparation changes first. After reviewing CI:

```sh
git tag -a v0.1.0-alpha.1 -m "kubewiser initial alpha"
git push origin v0.1.0-alpha.1
```

The tag workflow tests all four runners, packages the binaries, verifies checksums, and creates a **draft** GitHub Release. Review assets, limitations and native test results before publishing the draft in GitHub. A prerelease tag also marks the release as a prerelease. Existing releases are not overwritten automatically. The current feature set is an initial alpha; see README for inspection coverage.

## Installation without Go

Download the matching archive and `checksums.txt` from the same release. Verify the chosen archive against its entry (or download all four archives to verify the entire checksum list):

```sh
# Linux: compare this output with the corresponding checksums.txt entry.
sha256sum kubewiser_v0.1.0-alpha.1_linux_amd64.tar.gz
# macOS equivalent:
shasum -a 256 kubewiser_v0.1.0-alpha.1_darwin_arm64.tar.gz
```

Extract the matching archive, then install the executable:

```sh
tar -xzf kubewiser_v0.1.0-alpha.1_linux_amd64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 0755 kubewiser "$HOME/.local/bin/kubewiser"
export PATH="$HOME/.local/bin:$PATH"
kubewiser --version
```

Use the macOS archive name when installing on macOS. Persist the PATH addition in your shell configuration if necessary. Unsigned macOS downloads may require explicit approval in System Settings > Privacy & Security; verify the source and checksum first. Signing/notarization and Homebrew distribution are future work, not part of these archives.

For MCP, configure the client to launch the installed absolute executable path with `mcp --input /absolute/path/to/manifests`. No Go installation is required on the client machine.
