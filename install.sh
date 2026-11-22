#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if (( EUID == 0 )); then
	SUDO=()
else
	command -v sudo >/dev/null 2>&1 || { echo "Se requiere sudo para instalar Ameriboard." >&2; exit 1; }
	SUDO=(sudo -A)
fi

for tool in npm go systemctl install mktemp; do
	command -v "$tool" >/dev/null 2>&1 || { echo "No se encontró $tool." >&2; exit 1; }
done
if [[ ! -d /run/systemd/system ]]; then
	echo "Se requiere un sistema iniciado con systemd." >&2
	exit 1
fi

cd "$ROOT_DIR"
npm ci
npm run build
binary="$(mktemp)"
trap 'rm -f -- "$binary"' EXIT
CGO_ENABLED=0 go build -trimpath -o "$binary" ./cmd/ameriboard

"${SUDO[@]}" install -d -m 0755 /etc/ameriboard
"${SUDO[@]}" install -d -m 0755 /usr/share/ameriboard
"${SUDO[@]}" install -d -m 0755 /usr/lib/ameriboard
if ! "${SUDO[@]}" test -e /etc/ameriboard/ameriboard.env; then
	"${SUDO[@]}" install -m 0600 packaging/ameriboard.env /etc/ameriboard/ameriboard.env
fi
if ! "${SUDO[@]}" test -e /etc/ameriboard/ameriboard.yaml; then
	stopped=0
	if "${SUDO[@]}" systemctl is-active --quiet ameriboard.service; then
		"${SUDO[@]}" systemctl stop ameriboard.service
		stopped=1
	fi
	if ! "${SUDO[@]}" "$binary" -init-config /etc/ameriboard/ameriboard.yaml -legacy-env /etc/ameriboard/ameriboard.env; then
		if (( stopped )); then
			"${SUDO[@]}" systemctl start ameriboard.service
		fi
		exit 1
	fi
fi
"${SUDO[@]}" install -m 0755 "$binary" /usr/bin/ameriboard
"${SUDO[@]}" install -m 0644 packaging/ameriboard.service /etc/systemd/system/ameriboard.service
"${SUDO[@]}" install -m 0644 packaging/apache-ameriboard.conf.example packaging/nginx-ameriboard.conf.example /usr/share/ameriboard/
"${SUDO[@]}" install -m 0755 packaging/configure-proxy.sh /usr/lib/ameriboard/configure-proxy.sh
"${SUDO[@]}" systemctl daemon-reload
"${SUDO[@]}" systemctl enable ameriboard.service
if ! "${SUDO[@]}" systemctl restart ameriboard.service || ! "${SUDO[@]}" systemctl is-active --quiet ameriboard.service; then
	echo "No se pudo iniciar Ameriboard. Revisa: sudo journalctl -u ameriboard.service -b" >&2
	exit 1
fi
"${SUDO[@]}" env AMERIBOARD_PROXY="${AMERIBOARD_PROXY:-auto}" /usr/lib/ameriboard/configure-proxy.sh

echo "Ameriboard instalado. Configuración: /etc/ameriboard/ameriboard.yaml"
echo "Consulte listen_addr en ese archivo para acceder directamente al servicio."
