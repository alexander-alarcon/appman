# appman

`appman` es un gestor de AppImages para Linux escrito en Go. Administra las AppImages de un directorio, detecta sus metadatos y (en versiones posteriores) sincroniza lanzadores `.desktop` e iconos para integrarlas con el menú de aplicaciones.

> Estado: **v0.2 en desarrollo** (scanner, logging, inspección de metadatos e `info`). La sincronización/registro (`sync/install/remove`) y la TUI están planificadas.

## Características

- **Escaneo** de `*.AppImage` en un directorio (no recursivo por defecto).
- **Niveles de verbosidad**: `-v`, `-vv`, `-vvv` con salida coloreada.
- **Inspección de metadatos** (v0.2): tipo de AppImage, offset del SquashFS, nombre, versión, icono, categorías y `.desktop` embebido, con indicación del origen de cada dato. Extracción sin ejecutar la AppImage.
- CLI basada en Cobra. Directorio por defecto: `~/Applications/AppImages` (configurable con `--dir`).

## Instalación / compilación

Requisitos: **Go 1.27+** (según `go.mod`).

```bash
git clone <repo-url> appman
cd appman
go build ./cmd/appman
# binario: ./appman
```

Opcional, instalar en tu `$GOBIN`:

```bash
go install ./cmd/appman
```

## Uso

```bash
# Ayuda
./appman --help

# Escanear / listar AppImages
./appman scan
./appman list
./appman scan --dir /ruta/a/AppImages

# Verbosidad
./appman list -v      # rutas
./appman list -vv     # mtime/modo
./appman list -vvv    # debug

# Información detallada de una AppImage (v0.2)
./appman info <nombre|archivo>
./appman info --dir /ruta/a/AppImages MiApp.AppImage -vvv
```

`info` busca por nombre/archivo dentro de `--dir`, detecta el tipo (p.ej. Type 2 / SquashFS), lee el `.desktop` interno y localiza el icono, mostrando el origen de los metadatos y cualquier degradación.

## Dependencias

### Módulos de Go

Gestionados en `go.mod` (se descargan con `go build` / `go mod tidy`):

- [spf13/cobra](https://github.com/spf13/cobra) — CLI, subcomandos y flags.
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) — estilos/colores en terminal.
- (Planificado) Bubble Tea y Bubbles para la TUI, y un parser TOML para configuración.

### Dependencias del sistema

- **`squashfs-tools`** (proporciona `unsquashfs`): requerido para la extracción de metadatos/iconos embebidos de AppImages **Type 2** en `appman info` (v0.2). `appman` localiza `unsquashfs` en el `PATH` y lo invoca con el offset del SquashFS **sin ejecutar la AppImage**. Si no está instalado, `info` degrada el resultado y muestra un aviso accionable.

Instalación según distribución:

```bash
# Debian / Ubuntu
sudo apt install squashfs-tools

# Fedora
sudo dnf install squashfs-tools

# Arch Linux
sudo pacman -S squashfs-tools
```

Verificación:

```bash
command -v unsquashfs
unsquashfs -version
```

## Estructura del proyecto

```text
cmd/appman/              # entrypoint CLI
internal/
  appimage/              # scanner, detección de tipo/offset, metadatos e iconos
  desktop/               # (planificado) parser/generador de .desktop y registry
  manager/               # (planificado) orquestación y plan de sync
  config/                # (planificado) configuración
  logging/               # logging con niveles y color
  tui/                   # (planificado) interfaz Bubble Tea
```

## Roadmap

- v0.1: scanner y detección de AppImages, CLI y logging con verbosidad. ✅
- v0.2: extracción de metadatos e iconos, `info`. 🚧
- v0.3: generación/registro de `.desktop` y sincronización idempotente.
- v0.4: CLI pulida y completions.
- v0.5: TUI (Bubble Tea).
- v0.6+: sincronización inteligente y watcher opcional.

## Licencia

Por definir.
