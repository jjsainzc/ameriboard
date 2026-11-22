#!/bin/sh
set -e

if [ "$1" != configure ]; then
    exit 0
fi

if [ ! -e /etc/ameriboard/ameriboard.yaml ]; then
    stopped=0
    if [ -d /run/systemd/system ] && systemctl is-active --quiet ameriboard.service; then
        systemctl stop ameriboard.service
        stopped=1
    fi
    if ! /usr/bin/ameriboard -init-config /etc/ameriboard/ameriboard.yaml \
        -legacy-env /etc/ameriboard/ameriboard.env; then
        if [ "$stopped" = 1 ]; then
            systemctl start ameriboard.service
        fi
        exit 1
    fi
fi

if [ ! -d /run/systemd/system ]; then
    echo "Aviso: systemd no está activo; inicie ameriboard.service cuando esté disponible." >&2
    exit 0
fi

systemctl daemon-reload
systemctl enable ameriboard.service
if ! systemctl restart ameriboard.service || ! systemctl is-active --quiet ameriboard.service; then
    echo "No se pudo iniciar Ameriboard; consulte journalctl -u ameriboard.service -b." >&2
    exit 1
fi

if ! /usr/lib/ameriboard/configure-proxy.sh; then
    echo "Aviso: no se pudo configurar el proxy; revise la configuración de Ameriboard." >&2
fi
