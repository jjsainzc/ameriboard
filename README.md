# Ameriboard

> **NOTA**: En desarrollo, no esta funcional todavia

Aplicación Angular servida por Go como un único binario, sin archivos web externos
en tiempo de ejecución.

## Desarrollo y compilación

Se necesitan Go 1.26.4 o posterior y una versión de Node.js compatible con Angular
22 (por ejemplo, Node.js 24). Desde la raíz del proyecto:

```bash
npm ci
npm run build
go test ./...
go build -o ameriboard ./cmd/ameriboard
./ameriboard
```

La aplicación está disponible en `http://127.0.0.1:8087/ameriboard/`. Angular se
compila en `internal/web/dist` y Go lo incorpora al binario: repita `npm run build`
antes de compilar Go después de modificar el frontend. Para trabajar únicamente
en la interfaz, `npm start` inicia el servidor de desarrollo de Angular en
`http://localhost:4200/`; su proxy envía `/api` al servidor Go en el puerto
8087 con el `BASE_PATH` predeterminado. Arranque también el servidor Go y, si
utiliza otro puerto, actualice `proxy.conf.json` para desarrollo.

## Configuración

El servicio carga `/etc/ameriboard/ameriboard.yaml`. Sin `-config`, el binario
usa los valores predeterminados y las variables de entorno indicadas abajo.
Cuando se utiliza un YAML, los campos presentes tienen prioridad sobre el
entorno.

| Campo YAML | Variable de entorno | Valor predeterminado |
| --- | --- | --- |
| `listen_addr` | `LISTEN_ADDR` | `127.0.0.1:8087` |
| `base_path` | `BASE_PATH` | `/ameriboard` |
| `title` | `DASHBOARD_TITLE` | `CONFERENCIA AMERINET` |
| `subtitle` | `DASHBOARD_SUBTITLE` | `Reflector YSF {ysf_reflector} y TGIF DMR TG {tgif_tg}` |
| `badge` | `DASHBOARD_BADGE` | `Reflector + TGIF DMR` |
| `ysf_reflector` | `YSF_REFLECTOR_ID` | `74001` |
| `tgif_talkgroup` | `TGIF_TG` | `7401` |
| `bridge_callsign` | `BRIDGE_CALLSIGN` | `HC1LNM` |
| `ysf_log_dir` | `YSF_LOG_DIR` | `/var/log/YSFReflector` |
| `dmr_log_dir` | `DMR_LOG_DIR` | `/var/log/mmdvm` |

Ejemplo: `BASE_PATH=/panel LISTEN_ADDR=127.0.0.1:8087 ./ameriboard`.
El proxy inverso, si se utiliza, debe conservar el prefijo en la ruta enviada a
Go. La verificación de salud está en `/healthz`; la API de telemetría está en
`<BASE_PATH>/api/reflector-status`.
El reporte de conectados YSF incluye las direcciones IP registradas por el
reflector; serán visibles para quienes puedan acceder al dashboard.
Los dos directorios de logs deben existir y ser legibles para el usuario del
servidor; si no, la API responde con error en lugar de informar datos vacíos.

## Instalación como servicio

En Linux con systemd, Node.js, npm y Go, desde el código fuente:

```bash
bash install.sh
```

Ejecútelo como root o con sudo y un programa `SUDO_ASKPASS` configurado.
El instalador compila Angular y Go, instala el binario en `/usr/bin/ameriboard`
y habilita y reinicia `ameriboard.service`. En la primera instalación crea
`/etc/ameriboard/ameriboard.yaml`, migrando la configuración del archivo
`ameriboard.env` si existe. Prefiere el puerto configurado (8087 por defecto);
si está ocupado, selecciona un puerto libre y lo guarda en el YAML. El YAML
existente se conserva en instalaciones posteriores.
El servicio se ejecuta como root, como el de Amerimon, para poder leer los logs
configurados. Compruebe que los directorios de logs existan. Para cambiar el
puerto o los textos, edite el YAML y reinicie el servicio. Después de la
instalación no se elige otro puerto automáticamente: si está ocupado, el
servicio informa del error para que lo resuelva el administrador.

```bash
sudo systemctl restart ameriboard.service
sudo systemctl status ameriboard.service
sudo journalctl -u ameriboard.service -b
```

### Proxy inverso

El instalador detecta Apache o Nginx y configura el proxy automáticamente:
Apache en todos los sitios; Nginx en el servidor HTTP predeterminado cuando
puede identificarse sin ambigüedad. Valida la configuración antes de recargarla
y revierte los archivos gestionados si falla. Si ambos servidores están
instalados o activos y no hay uno preferente, seleccione explícitamente con
`AMERIBOARD_PROXY=apache bash install.sh` o `AMERIBOARD_PROXY=nginx bash install.sh`.
Use `AMERIBOARD_PROXY=none` para omitir el proxy.

El proxy utiliza `/run/ameriboard/proxy.sock`, independiente del puerto TCP:
tras cambiar `listen_addr` y reiniciar Ameriboard no es necesario modificar ni
recargar el proxy. La configuración automática requiere
`base_path: /ameriboard`. La regla `/ameriboard/ws` está reservada para un
WebSocket futuro; Ameriboard todavía no implementa ese endpoint.

## Distribución

Con GoReleaser v2 instalado y el proyecto en un repositorio Git, ejecute
`goreleaser release --snapshot --clean` para generar archivos `.tar.gz` y
`checksums.txt` para Linux amd64 y arm64 en `dist/`. GoReleaser instala las
dependencias bloqueadas de npm y recompila Angular antes de generar los binarios.
Para publicar una versión, cree un tag Git semántico y ejecute
`goreleaser release --clean` con la configuración de publicación y credenciales
correspondientes.

