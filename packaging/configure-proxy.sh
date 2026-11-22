#!/usr/bin/env bash
set -euo pipefail

if (( EUID != 0 )); then
	echo "La configuración del proxy requiere root." >&2
	exit 1
fi

apache=0
nginx=0
command -v apache2ctl >/dev/null 2>&1 && apache=1
command -v nginx >/dev/null 2>&1 && nginx=1
apache_active=0
nginx_active=0
(( apache )) && systemctl is-active --quiet apache2 && apache_active=1
(( nginx )) && systemctl is-active --quiet nginx && nginx_active=1

choice="${AMERIBOARD_PROXY:-auto}"
if [[ "$choice" == auto ]]; then
	if (( apache_active && nginx_active )); then
		echo "Apache y Nginx están activos; indique AMERIBOARD_PROXY=apache o nginx." >&2
		exit 1
	elif (( apache_active )); then
		choice=apache
	elif (( nginx_active )); then
		choice=nginx
	elif (( apache && nginx )); then
		echo "Apache y Nginx están instalados; indique AMERIBOARD_PROXY=apache o nginx." >&2
		exit 1
	elif (( apache )); then
		choice=apache
	elif (( nginx )); then
		choice=nginx
	else
		echo "No se detectó Apache ni Nginx; Ameriboard queda disponible directamente."
		exit 0
	fi
fi
case "$choice" in
	apache) (( apache )) || { echo "Apache no está instalado." >&2; exit 1; } ;;
	nginx) (( nginx )) || { echo "Nginx no está instalado." >&2; exit 1; } ;;
	none) echo "Configuración automática de proxy desactivada."; exit 0 ;;
	*) echo "AMERIBOARD_PROXY debe ser auto, apache, nginx o none." >&2; exit 1 ;;
esac

/usr/bin/ameriboard -config /etc/ameriboard/ameriboard.yaml -check-proxy
tempdir="$(mktemp -d)"
trap 'rm -f -- "$tempdir/dump" "$tempdir/original" "$tempdir/snippet"; rmdir "$tempdir"' EXIT

if [[ "$choice" == apache ]]; then
	target=/etc/apache2/conf-available/ameriboard.conf
	source=/usr/share/ameriboard/apache-ameriboard.conf.example
	[[ ! -e "$target" ]] || grep -Fqx '# Gestionado por Ameriboard.' "$target" || {
		echo "Existe una configuración Apache no gestionada en $target; no se modificó." >&2
		exit 1
	}
	apache2ctl -t -D DUMP_INCLUDES >"$tempdir/dump" 2>&1
	while IFS= read -r file; do
		[[ "$file" == "$target" || "$file" == /etc/apache2/conf-enabled/ameriboard.conf ]] && continue
		if grep -Eq '^[[:space:]]*(ProxyPass|ProxyPassMatch|RedirectMatch|<Location)[[:space:]].*/ameriboard([/[:space:]$>]|$)' "$file"; then
			echo "Ya existe una ruta /ameriboard en $file; no se modificó Apache." >&2
			exit 1
		fi
	done < <(awk '/^[[:space:]]*\([0-9]+\)[[:space:]]+\// {print $2}' "$tempdir/dump")
	had_file=0
	had_enabled=0
	[[ -e "$target" ]] && { cp -p "$target" "$tempdir/original"; had_file=1; }
	[[ -e /etc/apache2/conf-enabled/ameriboard.conf ]] && had_enabled=1
	rollback_apache() {
		if (( had_file )); then cp -p "$tempdir/original" "$target"; else rm -f -- "$target"; fi
		if (( ! had_enabled )) && [[ -e /etc/apache2/conf-enabled/ameriboard.conf ]]; then
			a2disconf ameriboard >/dev/null
		fi
	}
	install -m 0644 "$source" "$target"
	if ! a2enmod proxy proxy_http proxy_wstunnel headers || ! a2enconf ameriboard || ! apache2ctl configtest; then
		rollback_apache
		echo "Falló la validación de Apache; se restauró la configuración de Ameriboard." >&2
		exit 1
	fi
	if (( apache_active )) && ! systemctl reload apache2; then
		rollback_apache
		echo "Falló la recarga de Apache; se restauró la configuración de Ameriboard." >&2
		exit 1
	fi
	echo "Proxy global Apache configurado para /ameriboard/."
else
	source=/usr/share/ameriboard/nginx-ameriboard.conf.example
	snippet=/etc/nginx/snippets/ameriboard.conf
	nginx -T >"$tempdir/dump" 2>&1 || { echo "La configuración actual de Nginx no es válida." >&2; exit 1; }
	target="$(awk '
		/^# configuration file / { config=$4; sub(/:$/, "", config); next }
		{ line=$0; sub(/#.*/, "", line) }
		line ~ /^[[:space:]]*listen[[:space:]]/ && line ~ /(^|[[:space:]:])80([;[:space:]]|$)/ {
			if (first == "") first=config
			if (line ~ /default_server/ && default_file == "") default_file=config
		}
		END { if (default_file != "") print default_file; else if (first != "") print first; else exit 1 }
	' "$tempdir/dump")" || { echo "No se encontró un servidor HTTP en Nginx." >&2; exit 1; }
	target="$(readlink -f "$target")"
	[[ -f "$target" ]] || { echo "No se encontró el archivo de servidor Nginx." >&2; exit 1; }
	[[ ! -e "$snippet" ]] || grep -Fqx '# Gestionado por Ameriboard.' "$snippet" || {
		echo "Existe un snippet Nginx no gestionado en $snippet; no se modificó." >&2
		exit 1
	}
	if ! grep -Fq "include $snippet;" "$target"; then
		server_count="$(grep -Ec '^[[:space:]]*server[[:space:]]*\{' "$target" || true)"
		if [[ "$server_count" != 1 ]] || grep -Eq '^[[:space:]]*location[[:space:]].*/ameriboard' "$target"; then
			echo "No se puede seleccionar un único servidor Nginx sin conflicto en $target." >&2
			exit 1
		fi
	fi
	cp -p "$target" "$tempdir/original"
	had_snippet=0
	[[ -e "$snippet" ]] && { cp -p "$snippet" "$tempdir/snippet"; had_snippet=1; }
	install -d -m 0755 /etc/nginx/snippets
	install -m 0644 "$source" "$snippet"
	rollback_nginx() {
		cp -p "$tempdir/original" "$target"
		if (( had_snippet )); then cp -p "$tempdir/snippet" "$snippet"; else rm -f -- "$snippet"; fi
	}
	if ! grep -Fq "include $snippet;" "$target"; then
		target_tmp="$(mktemp "${target}.ameriboard.XXXXXX")"
		cp -p "$target" "$target_tmp"
		if ! awk -v snippet="$snippet" '
			!inserted && /^[[:space:]]*listen[[:space:]]/ && /(^|[[:space:]:])80([;[:space:]]|$)/ {
				print; print "    include " snippet ";"; inserted=1; next
			}
			{ print }
			END { if (!inserted) exit 1 }
		' "$target" >"$target_tmp"; then
			rm -f -- "$target_tmp"
			rollback_nginx
			echo "No se pudo insertar la inclusión en Nginx." >&2
			exit 1
		fi
		mv "$target_tmp" "$target"
	fi
	if ! nginx -t; then
		rollback_nginx
		echo "La configuración de Nginx no es válida; se restauró la original." >&2
		exit 1
	fi
	if (( nginx_active )) && ! systemctl reload nginx; then
		rollback_nginx
		echo "Falló la recarga de Nginx; se restauró la configuración original." >&2
		exit 1
	fi
	echo "Proxy Nginx configurado en $target para /ameriboard/."
fi
