# Ameriboard

> **NOTA**: Si hay alguna referencia al file que se Makefile, este fue retirado del codigo porque no es necesario para hacer compilaciones locales. 
> esta es una primera version Beta funcional:
> [Click aqui para ver dashboards digitales/echolink de Amerinet](http://159.203.177.35/ameriboard/)

Aplicación Angular servida por Go como un único binario, sin archivos web externos
en tiempo de ejecución.

Una práctica muy recoemndada es poner en un mismo server estos tres softwares de modo que se pueda optimizar el funcionamiento de los mismos, la opción de hacer puentes en DVSwitch para las diferentes modalidades consistes en crear directorios de MMDVM_BRIDGE con sufijos que los identifiquen y configurar los puentes de las stanzas deseadas invirtiendo los puertos tx y rx. 

Maneja tres logs fundamentales mas populares, YSF, Echolink (instalado en linux con TLB) y DVSwitch Server, sección MMDVM_BRIDGE, detecta puentes entre de modalidades con el criterio explicado anteriormente.

## Instalación

Paquetes Debian disponibles para `arm64` y `amd64`, en ese repositorio ya están compilados y listos para ser instalados:

```bash
curl -fsSL https://raw.githubusercontent.com/jjsainzc/repo-deb/main/hc1lnm.gpg \
  | sudo gpg --dearmor --yes -o /usr/share/keyrings/hc1lnm-archive-keyring.gpg

echo "deb [signed-by=/usr/share/keyrings/hc1lnm-archive-keyring.gpg arch=$(dpkg --print-architecture)] https://jjsainzc.github.io/repo-deb/ ./" \
  | sudo tee /etc/apt/sources.list.d/amerinet.list

sudo apt update
sudo apt install ameriboard
```

Para Debian 10 que ya no existe soporte, se puede usar este comando solo para actualizar la aplicación.
```
sudo apt install --only-upgrade ameriboard
```

> **NOTA:** Si lo va a usar en Raspberries o Thin Clients (ej. Dell Wyse), recomiendo use la instalación previamente descrita.
> Si usted es desarrollador con conocimientos de Node.js/Angular/Go Language entonces este repositorio le puede servir.

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
El pie de página muestra la versión incorporada al binario por GoReleaser;
en compilaciones locales sin versión indicada muestra «desarrollo».

## Configuración

El servicio carga `/etc/ameriboard/ameriboard.yaml`. Sin `-config`, el binario
usa los valores predeterminados y las variables de entorno indicadas abajo.
Cuando se utiliza un YAML, los campos presentes tienen prioridad sobre el
entorno.

| Campo YAML | Variable de entorno | Valor predeterminado |
| --- | --- | --- |
| `listen_addr` | `LISTEN_ADDR` | `127.0.0.1:8087` |
| `base_path` | `BASE_PATH` | `/ameriboard` |
| `title` | `DASHBOARD_TITLE` | `Ameriboard` |
| `subtitle` | `DASHBOARD_SUBTITLE` | `Panel de monitoreo de YSF, EchoLink y DVSwitch` |
| `badge` | `DASHBOARD_BADGE` | vacío |
| `ysf_reflector` | `YSF_REFLECTOR_ID` | `74001` |
| `tgif_talkgroup` | `TGIF_TG` | `7401` |
| `bridge_callsign` | `BRIDGE_CALLSIGN` | vacío (compatibilidad) |
| `ysf_log_dir` | `YSF_LOG_DIR` | `/var/log/YSFReflector` |
| `dmr_log_dir` | `DMR_LOG_DIR` | `/var/log/mmdvm` |
| `echolink_command` | `ECHOLINK_COMMAND` | `tlbcmd` |

Ejemplo: `BASE_PATH=/panel LISTEN_ADDR=127.0.0.1:8087 ./ameriboard`.
Los badges de puentes configurados se obtienen de `DVSwitch.ini` en los
directorios `/opt/MMDVM_Bridge*`: dos modalidades forman un puente cuando sus
puertos `txPort` y `rxPort` están invertidos en el mismo archivo. Se reconocen
DMR, D-Star (`DSTAR`), NXDN, P25 y YSF; se ignoran `Quantar_Bridge`, la
sección `QUANTAR` y las líneas comentadas.
Esto identifica una configuración, no confirma que el puente esté ejecutándose.
El estado de un puente no se infiere a partir de las conexiones YSF.
El proxy inverso, si se utiliza, debe conservar el prefijo en la ruta enviada a
Go. La verificación de salud está en `/healthz`; la API de telemetría está en
`<BASE_PATH>/api/reflector-status`.
El reporte de conectados YSF incluye las direcciones IP registradas por el
reflector; serán visibles para quienes puedan acceder al dashboard.
Los directorios de logs deben existir y ser legibles para el usuario del
servidor. La tabla DVSwitch toma la modalidad del registro de cada transmisión
y solo presenta las modalidades que aparecen en el log configurado en
`dmr_log_dir`. Acepta transmisiones locales (`Begin TX`/`TX state = OFF`) y de
red (`received ...`/`end of transmission`). Nombre y origen se muestran como
`—` si el log no los proporciona; pérdida y BER se completan cuando están presentes.
La fecha y hora DVSwitch se muestran en formato día/mes/año y en la zona
horaria del servidor que genera los datos.
Las tablas de conexiones y actividad muestran hasta 25 registros por página.
Cuando hay más de 25 registros se habilitan la búsqueda y la navegación; se
puede elegir entre 5, 10 y 25 registros por página. La búsqueda de actividad
DVSwitch está disponible también con menos registros. La actividad YSF y DVSwitch
comprende los registros disponibles de las últimas 24 horas; YSF conserva la
transmisión más reciente de cada indicativo.
Cuando falta una fuente, el dashboard oculta sus tablas y tarjetas sin mostrar
avisos; las demás fuentes continúan funcionando. Los últimos escuchados YSF
pueden mostrarse con datos del log DVSwitch, aun sin el log del reflector YSF.
Si EchoLink no está disponible se ocultan ambas tablas, pero se conserva el
historial guardado. Su leyenda de atributos está disponible junto a la
tabla cuando EchoLink responde. El dashboard también conserva los últimos
indicativos desconectados, la fecha y hora de desconexión y la duración de la
sesión en `echolink-history.json` dentro de `--state-dir` (por defecto
`/var/lib/ameriboard`). La desconexión se detecta en el siguiente sondeo, cada
5 segundos; la duración se cuenta desde la primera lectura que observó la
conexión. El archivo conserva el inicio de sesiones activas para continuar el
cálculo si se reinicia el servicio.

Tras iniciar sesión como administrador con el botón **Admin**, el menú
**Administración** permite gestionar sitios o cerrar sesión. En la tabla de
conectados EchoLink se puede silenciar o reactivar audio (rojo indica silenciado,
verde indica audio habilitado), desconectar y banear cada estación. Las opciones
generales permiten conectar, enviar mensajes y consultar la lista de baneados
para desbanear indicativos. Los mensajes no admiten punto y coma ni saltos de
línea, para evitar que se interpreten como comandos adicionales.
Para acciones en perfiles remotos se requiere la contraseña
de administrador del sitio remoto; los tokens antiguos solo permiten consultar
telemetría. Las solicitudes y los resultados de `tlbcmd` se registran en el
servidor que ejecuta la acción; para perfiles remotos, revisa también los
registros del servidor remoto. Ameriboard no registra deliberadamente las
credenciales ni el texto de los mensajes. La respuesta se omite para mensajes y cuando hay argumentos
adicionales en el comando configurado. En los demás casos puede contener
indicativos u otros datos sensibles del servicio: revísala antes de compartirla.
Un proceso finalizado no garantiza por sí solo que theBridge haya aceptado la
operación; revisa la respuesta registrada.

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

Después de iniciar sesión, la sección **Configuración principal** permite editar
el título y el subtítulo del dashboard local, así como el título de la tabla
DVSwitch. Los cambios se aplican sin reiniciar el servicio y se guardan en el
directorio de estado de Ameriboard; prevalecen sobre los valores del YAML o del
entorno, sin modificar esos archivos.

Para añadir un sitio remoto, introduce su URL HTTP o HTTPS y su contraseña de
administrador. Si la URL solo contiene el host, se añade automáticamente
`/ameriboard`; también puedes indicar una ruta personalizada. Obtén esa contraseña del responsable del sitio por un canal
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
su cabecera en este dashboard, además de un título para la tabla DVSwitch. Los
campos vacíos conservan los valores del sitio remoto; el estado del puente y la
última lectura siempre proceden de ese sitio. Al personalizar el subtítulo se
añade la última lectura del sitio. Estos cambios no modifican la configuración
remota.
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

Para seguir las acciones de EchoLink durante una prueba:

```bash
sudo journalctl -u ameriboard.service -f
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

> **NOTA**: El Makefile fue retirado del repositorio, entonces esta sección no es relevante, se deja por informacón solamente

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

