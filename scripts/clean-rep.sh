#!/usr/bin/env bash
set -euo pipefail

repo_dir="${REPO_DIR:-/var/www/html/repo}"
cd "$repo_dir"

for arch in amd64 arm64; do
    mapfile -t packages < <(find . -maxdepth 1 -type f -name "ameriboard_*_linux_${arch}.deb" -printf '%f\n' | sort -Vr)
    for package in "${packages[@]:3}"; do
        echo "Borrando: $package"
        rm -- "$package"
    done
done
