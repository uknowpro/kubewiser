# Releases

End users do not need Go, Helm, or kubectl for the currently implemented offline CLI and MCP tool. Build maintainers need Go (pinned in `.go-version`), Git, Bash, tar, zip and a SHA-256 utility.

## Platforms

| OS | Architecture | Archive suffix |
| --- | --- | --- |
| Linux | Intel/AMD 64-bit | linux_amd64 |
| Linux | ARM64 | linux_arm64 |
| macOS | Intel | darwin_amd64 |
| macOS | Apple Silicon | darwin_arm64 |
| Windows 10+ | Intel/AMD 64-bit | windows_amd64 (ZIP) |
| Windows 10+ | Intel/AMD 32-bit | windows_386 (ZIP) |

Builds use `CGO_ENABLED=0`; Linux binaries do not require a system libc. OS compatibility still follows the pinned Go toolchain's supported OS versions. macOS binaries are not Developer ID signed or notarized. Cross-compilation alone is not a native runtime test. CI is configured to run tests on the four Linux/macOS runners plus Windows Server 2022.

## Local preparation

```sh
go test -race ./...
go vet ./...
bash scripts/release.sh v0.1.0-alpha.5
```

Outputs are under `dist/v0.1.0-alpha.5/`: four tar.gz archives and two Windows ZIPs containing the executable, LICENSE and README, plus SHA-256 checksums and build metadata. Existing output directories are rejected. Build from a clean checkout for a release; archives embed the version and Git commit. Archives are not claimed to be byte-for-byte reproducible.

## GitHub release

Commit and push the release preparation changes first. After reviewing CI:

```sh
git tag -a v0.1.0-alpha.5 -m "kubewiser initial alpha"
git push origin v0.1.0-alpha.5
```

The tag workflow tests all six test jobs, packages the binaries, verifies checksums, and creates a **draft** GitHub Release. Review assets, limitations and native test results before publishing the draft in GitHub. A prerelease tag also marks the release as a prerelease. Existing releases are not overwritten automatically. The current feature set is an initial alpha; see README for inspection coverage.

## Installation without Go

Download the matching archive and `checksums.txt` from the same release. Verify the chosen archive against its entry (or download all six archives to verify the entire checksum list):

```sh
# Linux: compare this output with the corresponding checksums.txt entry.
sha256sum kubewiser_v0.1.0-alpha.5_linux_amd64.tar.gz
# macOS equivalent:
shasum -a 256 kubewiser_v0.1.0-alpha.5_darwin_arm64.tar.gz
```

Extract the matching archive, then install the executable:

```sh
tar -xzf kubewiser_v0.1.0-alpha.5_linux_amd64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 0755 kubewiser "$HOME/.local/bin/kubewiser"
export PATH="$HOME/.local/bin:$PATH"
kubewiser --version
```

Use the macOS archive name when installing on macOS. Persist the PATH addition in your shell configuration if necessary. Unsigned macOS downloads may require explicit approval in System Settings > Privacy & Security; verify the source and checksum first. Signing/notarization and Homebrew distribution are future work, not part of these archives.

For MCP, configure the client to launch the installed absolute executable path with `mcp --input /absolute/path/to/manifests`. No Go installation is required on the client machine.

## Windows 10 x64 and x86

Windows 10 64-bit (Intel/AMD) is the target. Go's documented minimum is Windows 10 for Go 1.21 and later: https://go.dev/wiki/MinimumRequirements. CI executes on Windows Server 2022, not a Windows 10 desktop; Windows 10 device testing remains a separate validation step. Windows x86 (386) is also packaged and tested under WOW64 on the same runner; no native 32-bit Windows 10 device has been tested. Windows ARM64 binaries are not included.

Download the Windows ZIP and checksums.txt from the same release. In PowerShell:

```powershell
Get-FileHash .\kubewiser_v0.1.0-alpha.5_windows_amd64.zip -Algorithm SHA256
# Compare with the matching entry in checksums.txt before extraction.
Expand-Archive .\kubewiser_v0.1.0-alpha.5_windows_amd64.zip -DestinationPath .\kubewiser
cd .\kubewiser
.\kubewiser.exe --version
.\kubewiser.exe scan files --target-version 1.32 --format json "C:\deploy manifests"
.\kubewiser.exe mcp --input "C:\deploy manifests"
```

Use an absolute executable path in your MCP client configuration; JSON Windows paths require escaped backslashes (for example `C:\\Tools\\kubewiser\\kubewiser.exe`). No Go, WSL, Bash, Helm or kubectl installation is needed for the current executable. It is a console application: run it from PowerShell or CMD rather than double-clicking. The executable is not Authenticode-signed; Windows may display a SmartScreen prompt.

Before tagging a release, add bilingual notes at `docs/release-notes/<tag>.md`. The release workflow uses that file and fails instead of publishing without it.

## Alternate command and 32-bit Windows

Linux/macOS archives include both command names. Install both with `install -m 0755 kubewiser kubewise "$HOME/.local/bin/"`. Windows archives contain only `kubewiser.exe`; use a shell alias for the second name.

For Windows 10 32-bit, download `kubewiser_v0.1.0-alpha.5_windows_386.zip` and substitute that filename in the installation example above. Both names support the same flags and MCP behavior; version banners intentionally use the product name kubewiser. 32-bit support in this release applies to Windows x86, not macOS or Linux.

## Automatic Windows command registration

From the extracted ZIP, run `.\install.ps1` in PowerShell. It copies the single bundled EXE into `%LOCALAPPDATA%\kubewiser\versions\<content-hash>`, creates `kubewise.exe` as a hardlink to `kubewiser.exe`, and prepends that directory to user PATH and the current process PATH. No administrator privileges, profile edits, network requests, or duplicate executable data are needed. There are two visible names referencing the same file data; ZIP files still contain only one EXE.

Both `kubewiser --version` and `kubewise --version` then work in that PowerShell session and new child CMD processes. Restart existing terminal applications to refresh inherited PATH. Existing conflicting aliases or earlier system PATH commands can take precedence; inspect `Get-Command kubewise -All` / `where.exe kubewise` if necessary.

Hardlinks require a supported local filesystem and both names on the same volume; the default is normally on NTFS. Unsupported filesystems fail rather than silently creating a second executable copy. Existing unrelated executables are rejected. Re-running the same installer is idempotent. New binaries get a separate content-hash directory, so running older processes are not overwritten. Old version directories/PATH entries remain available for manual cleanup. `-InstallDir` can select a directory, but an existing different binary is never overwritten.

PowerShell execution policy may prevent downloaded scripts from running. Follow your organization's approved script-signing/unblocking process; the installer does not weaken policy. On locked-down systems the EXE can still be used directly.

To remove an installation, remove its directory from user PATH and delete that version directory once its processes have exited. Downloaded ZIPs and extracted source copies are separate from the installed hardlinked pair and may be removed after successful installation.

[Microsoft hardlink semantics](https://learn.microsoft.com/en-us/windows/win32/fileio/hard-links-and-junctions).
