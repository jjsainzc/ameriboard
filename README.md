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

La aplicación está disponible en `http://127.0.0.1:8082/dashboard/`. Angular se
compila en `internal/web/assets/dist` y Go lo incorpora al binario: repita `npm run build`
antes de compilar Go después de modificar el frontend. Para trabajar únicamente
en la interfaz, `npm start` inicia el servidor de desarrollo de Angular.

## Configuración

| Variable | Valor predeterminado | Descripción |
| --- | --- | --- |
| `LISTEN_ADDR` | `127.0.0.1:8082` | Dirección de escucha HTTP. |
| `BASE_PATH` | `/dashboard` | Prefijo público de la aplicación; use `/` para servirla desde la raíz. |

Ejemplo: `BASE_PATH=/panel LISTEN_ADDR=127.0.0.1:8082 ./ameriboard`.
El proxy inverso, si se utiliza, debe conservar el prefijo en la ruta enviada a
Go. La verificación de salud está en `/healthz`; la API de ejemplo está en
`<BASE_PATH>/api/reflector-status`.

## Distribución

Con GoReleaser v2 instalado y el proyecto en un repositorio Git, ejecute
`goreleaser release --snapshot --clean` para generar archivos `.tar.gz` y
`checksums.txt` para Linux amd64 y arm64 en `dist/`. GoReleaser instala las
dependencias bloqueadas de npm y recompila Angular antes de generar los binarios.
Para publicar una versión, cree un tag Git semántico y ejecute
`goreleaser release --clean` con la configuración de publicación y credenciales
correspondientes.
# ameriboard
