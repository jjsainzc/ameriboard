#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(mktemp -d)"
cleanup() {
    for file in "$repo"/*; do
        [ ! -e "$file" ] || rm -- "$file"
    done
    rmdir "$repo"
}
trap cleanup EXIT

for arch in amd64 arm64; do
    for version in 1.0.2 1.0.9 1.0.10 1.0.11; do
        touch "$repo/ameriboard_${version}_linux_${arch}.deb"
    done
done
touch "$repo/amerimon_1.0.1_linux_amd64.deb"
printf '#!/bin/sh\nset -e\ntouch indexed\n' > "$repo/update-repo.sh"
chmod +x "$repo/update-repo.sh"
make -C "$root" clean-repo REPO_DIR="$repo"

for arch in amd64 arm64; do
    [ ! -e "$repo/ameriboard_1.0.2_linux_${arch}.deb" ]
    for version in 1.0.9 1.0.10 1.0.11; do
        [ -f "$repo/ameriboard_${version}_linux_${arch}.deb" ]
    done
done
[ -f "$repo/amerimon_1.0.1_linux_amd64.deb" ]
[ -f "$repo/indexed" ]
echo "Retención de paquetes verificada."
