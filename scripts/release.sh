#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
version="${1:-}"
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: bash scripts/release.sh v0.1.0-alpha.1" >&2
  exit 2
fi
commit="$(git rev-parse HEAD)"
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  commit="${commit}-dirty"
fi
output="dist/$version"
if [[ -e "$output" ]]; then
  echo "Output already exists: $output (choose a new version or move it aside)" >&2
  exit 2
fi
mkdir -p dist
stage="$(mktemp -d dist/.release.XXXXXX)"
trap 'rm -rf "$stage"' EXIT
export CGO_ENABLED=0
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/386; do
  os="${target%/*}"
  arch="${target#*/}"
  name="kubewiser_${version}_${os}_${arch}"
  alias=kubewise
  binary=kubewiser
  if [[ "$os" == windows ]]; then binary=kubewiser.exe; alias=kubewise.exe; fi
  mkdir -p "$stage/$name"
  GOOS="$os" GOARCH="$arch" go build -mod=readonly -trimpath \
    -ldflags="-s -w -X github.com/uknowpro/kubewiser/internal/buildinfo.Version=$version -X github.com/uknowpro/kubewiser/internal/buildinfo.Commit=$commit" \
    -o "$stage/$name/$binary" ./cmd/kubewiser
  if [[ "$os" != windows ]]; then cp "$stage/$name/$binary" "$stage/$name/$alias"; fi
  cp LICENSE README.md README.ko.md THIRD_PARTY_NOTICES.txt "$stage/$name/"
  if [[ "$os" == windows ]]; then
    cp scripts/install-windows.ps1 "$stage/$name/install.ps1"
    (cd "$stage/$name" && zip -q "../$name.zip" "$binary" install.ps1 LICENSE README.md README.ko.md THIRD_PARTY_NOTICES.txt)
  else
    tar -czf "$stage/$name.tar.gz" -C "$stage/$name" "$binary" "$alias" LICENSE README.md README.ko.md THIRD_PARTY_NOTICES.txt
  fi
  rm -rf "$stage/$name"
done
(
  cd "$stage"
  if command -v sha256sum >/dev/null; then
    sha256sum ./*.tar.gz ./*.zip > checksums.txt
  else
    shasum -a 256 ./*.tar.gz ./*.zip > checksums.txt
  fi
)
printf 'version=%s\ncommit=%s\ntoolchain=%s\ncgo=0\n' "$version" "$commit" "$(go version)" > "$stage/build-info.txt"
mv "$stage" "$output"
printf 'Release assets: %s\n' "$output"
