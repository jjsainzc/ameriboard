#!/bin/sh
set -e

case "$1" in
    remove|deconfigure)
        if [ -d /run/systemd/system ]; then
            systemctl stop ameriboard.service
            systemctl disable ameriboard.service
        fi
        ;;
esac
