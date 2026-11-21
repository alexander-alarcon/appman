# appman

`appman` es un gestor de AppImages para Linux escrito en Go. Administra las AppImages de un directorio, detecta sus metadatos y sincroniza lanzadores `.desktop` e iconos para integrarlas con el menú de aplicaciones.

> Estado: **v0.4 en desarrollo** (CLI pulida, completions bash/zsh/fish y robustez). La TUI está planificada.

## Características

- **Escaneo** de `*.AppImage` en un directorio (no recursivo por defecto).
- **Niveles de verbosidad**: `-v`, `-vv`, `-vvv` con salida coloreada.
- **Inspección de metadatos**: tipo de AppImage, offset del SquashFS, nombre, versión, icono, categorías y `.desktop` embebido, con indicación del origen de cada dato. Extracción sin ejecutar la AppImage.
- **Integración de escritorio**: genera/actualiza entradas `.desktop` gestionadas e iconos para las AppImages detectadas, con plan de sincronización idempotente (estado real vs deseado) y sin tocar lanzadores no gestionados.
- **Gestión de huérfanos**: detecta entradas gestionadas cuyo AppImage ya no existe; se eliminan solo bajo petición explícita (`--prune`).
- **Completions** para bash, zsh y fish (incluye sugerencias dinámicas de apps para `info`, `install` y `remove`).
- CLI basada en Cobra. Directorio por defecto: `~/Applications/AppImages` (configurable con `--dir`; MVP sin archivo de configuración).

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

# Escanear / listar AppImages e integración
./appman scan
./appman list
./appman list --installed
./appman list --available
./appman list --orphaned
./appman scan --dir /ruta/a/AppImages

# Verbosidad
./appman list -v      # rutas
./appman list -vv     # mtime/modo
./appman list -vvv    # debug

# Información detallada de una AppImage
./appman info <nombre|archivo>
./appman info --dir /ruta/a/AppImages MiApp.AppImage -vvv

# Sincronizar integraciones (.desktop e iconos gestionados)
./appman sync --dry-run
./appman sync
./appman sync --prune        # además elimina huérfanos gestionados

# Integrar o quitar una AppImage concreta
./appman install <app>
./appman remove <app>
```

- `info` busca por nombre/archivo dentro de `--dir`, detecta el tipo (p.ej. Type 2 / SquashFS), lee el `.desktop` interno y localiza el icono, mostrando el origen de los metadatos y cualquier degradación.
- `sync`/`install`/`remove` calculan un plan (crear/actualizar/eliminar/sin cambios) y solo aplican lo necesario; repetir `sync` sin cambios es idempotente. Los huérfanos gestionados no se eliminan salvo con `--prune`.

## Configuración (MVP)

En esta versión la configuración se hace por **flags** (sin archivo de configuración):

- `--dir`: directorio de AppImages (default: `~/Applications/AppImages`).
- `-v, -vv, -vvv`: verbosidad.

Las entradas `.desktop` creadas por `appman` se marcan como gestionadas (`X-Appman-Managed`, `X-Appman-ID`, `X-Appman-AppImage`) para no modificar ni eliminar lanzadores ajenos.

## Completions

`appman` genera scripts de autocompletado para bash, zsh y fish:

```bash
# Bash
source <(appman completion bash)

# Zsh
source <(appman completion zsh)

# Fish
appman completion fish > ~/.config/fish/completions/appman.fish
```

Los comandos `info`, `install` y `remove` sugieren dinámicamente nombres/archivos de AppImages detectados en `--dir`.

## Dependencias

### Módulos de Go

Gestionados en `go.mod` (se descargan con `go build` / `go mod tidy`):

- [spf13/cobra](https://github.com/spf13/cobra) — CLI, subcomandos y flags.
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) — estilos/colores en terminal.
- (Planificado) Bubble Tea y Bubbles para la TUI.

### Dependencias del sistema

- **`squashfs-tools`** (proporciona `unsquashfs`): requerido para la extracción de metadatos/iconos embebidos de AppImages **Type 2** (`info`, `list`/`sync` cuando inspeccionan metadatos). `appman` localiza `unsquashfs` en el `PATH` y lo invoca con el offset del SquashFS **sin ejecutar la AppImage**. Si no está instalado, la extracción degrada el resultado y se muestra un aviso accionable.

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
  cli/                   # comandos, wiring y presentación de la CLI
  desktop/               # parser/generador de .desktop y listado de entradas gestionadas
  manager/               # estado, plan de sync y aplicación de cambios
  config/                # defaults de configuración (MVP por flags)
  logging/               # logging con niveles y color
  tui/                   # (planificado) interfaz Bubble Tea
```

## Roadmap

- v0.1: scanner y detección de AppImages, CLI y logging con verbosidad. ✅
- v0.2: extracción de metadatos e iconos, `info`. ✅
- v0.3: generación/registro de `.desktop` y sincronización idempotente. ✅
- v0.4: CLI pulida, completions (bash/zsh/fish) y robustez. 🚧
- v0.5: TUI (Bubble Tea).
- v0.6+: sincronización inteligente y watcher opcional.

## Licencia

Por definir.
