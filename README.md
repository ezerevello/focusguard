# FocusGuard

A local website blocker with its own web UI for Linux and Windows.
No Docker, no Node.

<img width="1920" height="1080" alt="Screenshot_20260621_014050" src="https://github.com/user-attachments/assets/58cae1c8-b57b-4788-ad34-ceba0cea9661" />
<img width="1920" height="1080" alt="Screenshot_20260621_014111" src="https://github.com/user-attachments/assets/3050b497-a26b-440f-a25c-7164bef18a0d" />



This is a single Go binary, cross-compiled for Linux and Windows (`GOOS=windows go build`), which:

* Serves the embedded web UI (vanilla HTML/CSS/JS, no build step, completely self-contained in the binary via `go:embed`),
* Exposes a local REST API,
* Edits the system's hosts file to redirect blocked domains to `127.0.0.1`.

Zero external dependencies—the entire backend relies solely on the Go standard library.

## How It Blocks

When you block a site, FocusGuard appends entries to the hosts file (`/etc/hosts` on Linux, `C:\Windows\System32\drivers\etc\hosts` on Windows) within a dedicated block delimited by its own markers. This ensures it never touches the rest of your file:

```
# === FocusGuard START (automatically managed, do not edit manually) ===
127.0.0.1 youtube.com
127.0.0.1 www.youtube.com
::1 youtube.com
::1 www.youtube.com
# === FocusGuard END ===

```

This blocks both the website and any desktop app that communicates with those same domains—for example, **WhatsApp Desktop** (Electron) connects to `web.whatsapp.com`, so blocking that domain also cuts off the Windows app, not just WhatsApp Web.

### Limitations You Should Know About

* **DNS-over-HTTPS (DoH) in the browser can bypass the hosts file.** If Firefox has "Secure DNS" enabled, it sometimes resolves domains directly via HTTPS without consulting the system's hosts file. Chrome generally respects hosts file entries even with Secure DNS turned on, but if you notice a "blocked" site is still loading, the first thing to check is your browser's DoH settings.
* Apps that hardcode IP addresses instead of performing DNS lookups won't be blocked this way (this is rare, but it happens).
* The process needs to run as root/Administrator at all times to write to the hosts file. The setup script installs it as a service/task with these permissions.

## Installation

### Linux

```bash
git clone https://github.com/ezerevello/focusguard
cd focusguard
./scripts/setup.sh
```

This compiles the binary, installs it to `/usr/local/bin/focusguard`, and registers it as a systemd service (runs as root, starts automatically on boot). Once finished, it opens `http://focus.local/` (or `http://localhost/`) in your browser.

To uninstall: `./scripts/setup.sh --uninstall`

#### Without systemd

If your distro doesn't use systemd, compile and run the binary manually:

```bash
go build -o focusguard ./cmd/focusguard
sudo ./focusguard
```

### Windows

Open PowerShell **as Administrator** (or let the script self-elevate) and run:

```powershell
git clone https://github.com/ezerevello/focusguard
cd focusguard
.\scripts\setup.ps1
```

This compiles the binary (installing Go via `winget` if necessary), moves it to `Program Files\FocusGuard`, and registers a Scheduled Task that launches it with elevated privileges at every login.

Windows SmartScreen might warn you that the `.exe` is unsigned—this is expected for locally compiled binaries; click "More info" → "Run anyway".

> **Note:** This script hasn't been tested on a physical Windows machine yet (it was written and reviewed carefully, but the primary development environment is Linux). If anything goes wrong, please open an issue.

To uninstall: `.\scripts\setup.ps1 -Uninstall`

## Usage

Once installed, the web UI is available at `http://focus.local/` (or simply `http://localhost/`):

* **Sites**: Easily add services with a single click from presets (YouTube, WhatsApp, Instagram, TikTok, Facebook, X, Reddit, Netflix, Twitch, Discord) or add a custom domain. Each has an on/off toggle.
* **Focus Session**: Choose which sites to block and for how many minutes. While a session is active, these sites **cannot be unlocked** until the timer runs out—this friction is the core concept of the app.
* **Easy Access**: No need to type ports or `localhost`. The installer creates a native desktop/application shortcut and maps a custom local domain so you can access the web UI simply by navigating to `http://focus.local`.

## Project Structure

```
cmd/focusguard/        main.go + embedded web UI (web/static)
internal/hostsfile/    Secure hosts file editing (Linux/Windows)
internal/store/        JSON persistence (sites + focus sessions)
internal/api/          REST handlers
internal/presets/      Predefined domain lists
internal/sysutil/      Elevated privileges detection
scripts/setup.sh       Linux installer (systemd)
scripts/setup.ps1      Windows installer (Scheduled Task)
systemd/               Unit file
```

## Development

```bash
go build ./cmd/focusguard
go test ./...
go run ./cmd/focusguard -port 7878
```

State is saved in `~/.focusguard/data.json` (Linux/macOS) or `%ProgramData%\FocusGuard\data.json` (Windows). You can override this using the `-data /custom/path.json` flag, and change the port with `-port`.

## Security

The web UI listens only on `127.0.0.1`—it is not accessible from other machines on your network. Since the process runs with elevated privileges (required to write to the hosts file), do not expose it behind a proxy or change the bind address to `0.0.0.0` without adding authentication first.

## License

MIT — see [LICENSE](https://www.google.com/search?q=LICENSE).
