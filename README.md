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
| `echolink_command` | `ECHOLINK_COMMAND` | `tlbcmd` |

Ejemplo: `BASE_PATH=/panel LISTEN_ADDR=127.0.0.1:8087 ./ameriboard`.
El proxy inverso, si se utiliza, debe conservar el prefijo en la ruta enviada a
Go. La verificación de salud está en `/healthz`; la API de telemetría está en
`<BASE_PATH>/api/reflector-status`.
El reporte de conectados YSF incluye las direcciones IP registradas por el
reflector; serán visibles para quienes puedan acceder al dashboard.
Los directorios de logs deben existir y ser legibles para el usuario del
servidor. Si YSF o DMR no están disponibles, el dashboard muestra un aviso y
marca sus estadísticas como no disponibles sin bloquear las demás fuentes.
Cuando EchoLink no se puede consultar, su tabla se oculta y el error queda en
el registro del servicio. Su leyenda de atributos está disponible junto a la
tabla cuando EchoLink responde.

## Sitios remotos y perfiles

Cada instalación puede consultar otras instalaciones Ameriboard por HTTP o HTTPS. Un
administrador crea los perfiles con el nombre, la URL del dashboard remoto y
la contraseña del administrador de ese sitio. Cada perfil puede marcarse como visible para
visitantes; los perfiles ocultos solo aparecen con una sesión de administrador.
Los visitantes pueden consultar libremente los datos de los perfiles visibles.
La visibilidad controla el acceso desde esta instalación: no modifica los
permisos ni la publicación del dashboard en el servidor remoto.

**HTTP no cifra contraseñas, sesiones ni datos**: úsenlo solo en redes de
confianza. Se recomienda HTTPS con certificados válidos para cada servidor.
Las direcciones remotas deben resolver a IP públicas y, con HTTPS, usar
certificados de confianza. No se siguen
redirecciones ni se permite conectar a direcciones privadas o de loopback. El
servicio debe permanecer en loopback o detrás de un proxy confiable que
reemplace `X-Forwarded-Proto`, como los ejemplos incluidos.

En cada instalación, accede a **Administrar sitios** y crea la
contraseña inicial del administrador (entre 8 y 72 bytes). Hazlo inmediatamente:
hasta que exista un administrador, el primer visitante que complete el formulario
podrá crearlo. La contraseña local se guarda como hash; la sesión dura como
máximo ocho horas y la cookie no es persistente.

Para añadir un sitio remoto, introduce su URL HTTP o HTTPS y su contraseña de
administrador. Obtén esa contraseña del responsable del sitio por un canal
seguro: también permite administrar directamente ese sitio. Las contraseñas
remotas se almacenan en el servidor local para poder consultar sus datos; nunca
se muestran al volver a editar el perfil. Deja el campo vacío al editar para
conservar la contraseña existente.

Para recuperar el acceso, un administrador del servidor puede generar una
contraseña nueva con `sudo -A /usr/bin/ameriboard -generate-credential admin`.
El comando muestra esa contraseña una sola vez e invalida las sesiones
existentes. Al cambiar la contraseña de un sitio deben actualizarse los perfiles
que lo consultan desde otras instalaciones. Los perfiles anteriores que usen
tokens continúan funcionando. Las
credenciales y perfiles se guardan bajo `/var/lib/ameriboard` en archivos
accesibles únicamente al propietario del servicio. No incluya contraseñas en URLs.

Tras iniciar sesión, cree o edite los perfiles y elija su visibilidad.
Opcionalmente, cada perfil permite definir el título, subtítulo y etiqueta de
su cabecera en este dashboard. Los campos vacíos conservan los valores del
sitio remoto; el estado del puente y la última lectura siempre proceden de
ese sitio. Al personalizar el subtítulo se añade la última lectura del sitio.
Estos cambios no modifican la configuración remota.
Desde el selector del dashboard se puede volver al sitio local en cualquier
momento. Un sitio remoto inaccesible muestra un error solo para ese sitio; no
impide consultar los demás. Cuando una fuente de telemetría remota falla, el
sitio remoto devuelve los datos restantes junto con sus advertencias.
El botón **Refrescar datos** consulta inmediatamente el sitio seleccionado,
además de la actualización automática cada cinco segundos.

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

Con GoReleaser v2, Go, Node.js y npm instalados, `goreleaser release --snapshot
--clean` genera paquetes `.deb`, archivos `.tar.gz` y `checksums.txt` para
Linux amd64 y arm64 en `dist/`. GoReleaser instala las dependencias bloqueadas
de npm y recompila Angular antes de generar los binarios.

Para publicar tanto la versión de GitHub como los paquetes en el mismo repositorio
Debian de Amerimon:

```bash
make dist                     # siguiente patch; sin tags comienza en v1.0.1
make dist VERSION=v1.2.0      # versión explícita
make dist REPO_DIR=/ruta/repo # repositorio Debian alternativo
```

El árbol de Ameriboard debe estar limpio y se necesitan permisos y credenciales
para publicar en ambos repositorios, además de GoReleaser configurado para GitHub.
El directorio Debian debe ser un repositorio Git con `update-repo.sh` y disponer
de los permisos y la clave necesarios para firmar sus índices; debe estar en
la rama `main` sin cambios preparados ni cambios pendientes en los índices o
paquetes de Ameriboard. El proceso publica
la rama y el tag, genera la versión, copia los paquetes, conserva las tres
versiones más recientes **por arquitectura de Ameriboard**, regenera y firma los
índices, y publica el commit en `origin/main`. No elimina paquetes de Amerimon
ni de otros proyectos. La publicación también incluye los cambios pendientes en
el `README.md` del repositorio Debian; revíselos antes de ejecutar `make dist`.
La publicación puede fallar después de crear el tag;
revise el estado de ambos repositorios antes de reintentar cada paso.

`make` muestra la ayuda sin publicar. Los pasos `release`, `copy-repo`,
`clean-repo`, `index-repo` y `push-repo` también pueden ejecutarse por separado;
`clean-repo` elimina paquetes antiguos y regenera los índices.

Tras configurar el repositorio Debian en el equipo de destino, instale con
`sudo apt install ameriboard`. El paquete conserva el YAML y el archivo de
entorno existentes, inicia el servicio con systemd y procura configurar el
proxy como el instalador desde fuente. Si la selección automática del proxy
falla, se informa un aviso y puede ejecutarse
`sudo AMERIBOARD_PROXY=apache /usr/lib/ameriboard/configure-proxy.sh`
(o `nginx`/`none`) después de resolver el conflicto.

