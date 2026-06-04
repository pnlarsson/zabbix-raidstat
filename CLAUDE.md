# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`zabbix-raidstat` parses the output of vendor RAID CLI tools (`arcconf`, `ssacli`, `mvcli`, `megacli`, `sas2ircu`) and emits JSON for Zabbix low-level discovery (LLD) and item monitoring. It supports Adaptec/Microsemi, HP Smart Array, Lenovo M.2 (Marvell `mvcli`), LSI MegaRAID, and LSI `sas2ircu`.

## Build & Run

- `make` (default goal `compile`) — builds the `raidstat` binary plus one `.so` per vendor into `./build/`, and copies `config.json` there. The binary and plugins must sit in the same directory at runtime.
- `make tar` — produces `raidstat.tar.gz` with the `build/` contents renamed to `raidstat/`.
- `make install` — installs everything to `/opt/raidstat` (note: the `install`/`tar` targets depend on a `build` target that does not exist in the Makefile; run `make compile` first or fix the dependency).
- `make clean` — removes `./build` and `raidstat.tar.gz`.

Run examples (binary reads `config.json` from its own directory, resolved via `os.Executable()`):
```
./build/raidstat --vendor adaptec -d ct                 # discover controllers
./build/raidstat --vendor hp -s ld,0,1 -i 2             # logical drive status, indented
```
Set `RAIDSTAT_DEBUG=y` to print every executed command, its output, and regex matches (see `functions.go`) — the primary debugging tool.

## Testing

There is no Go test suite. Testing is done against mock data: `testdata/<vendor>.sh` are stub scripts that `cat` canned tool output from `testdata/<vendor>/*.txt` based on the CLI args.

`./testdata/run-tests.sh` (run after `make`) is an assertion-based regression harness: it points `build/config.json` at the mock tools, exercises the multi-vendor discovery/status flow as Zabbix would, asserts the output, and restores the shipped config on exit. Add a case there when fixing a parser. To exercise one parser ad-hoc, point its `config.json` entry at the matching `testdata/<vendor>.sh` and run `raidstat` from the repo root (the mocks use relative paths). When changing a parser's regexes, update the corresponding `testdata/<vendor>/*.txt` fixtures if the real tool output format differs.

## Architecture

The program is a Go **plugin host**. `main.go` is the host; each `plugins/<vendor>/main.go` is compiled with `-buildmode=plugin` into `<vendor>.so` and loaded at runtime by `plugin.Open(<vendor>.so)`.

- **`config.json`** maps a vendor name → its CLI binary (e.g. `"hp": "ssacli"`). The vendor name doubles as the plugin filename (`hp` → `hp.so`). Adding a vendor means adding a config entry, a `plugins/<vendor>/` package, and a Makefile rule.
- **`main.go`** parses CLI args (docopt), validates the vendor/operation against `config.json`, opens the matching `.so`, and dispatches. It looks up exported plugin symbols **by name with hardcoded type assertions** — the plugin contract is implicit, not a Go interface.
- Each plugin **must export** these symbols with exactly these signatures:
  - `GetControllersIDs(execPath string) []string`
  - `GetLogicalDrivesIDs(execPath, controllerID string) []string`
  - `GetPhysicalDrivesIDs(execPath, controllerID string) []string`
  - `GetControllerStatus(execPath, controllerID string, indent int) []byte`
  - `GetLDStatus(execPath, controllerID, deviceID string, indent int) []byte`
  - `GetPDStatus(execPath, controllerID, deviceID string, indent int) []byte`
  - and a no-op `func main() {}`
- **Discovery** functions return ID lists; `main.go` wraps them in Zabbix LLD JSON with macro keys (`{#CT_ID}`, `{#LD_ID}`, `{#PD_ID}`). **Status** functions return the final JSON `[]byte` themselves (status, model, temperature, etc.), with per-status normalization (e.g. `Optimal`/`Online` → `OK`).
- **`plugins/internal/functions/functions.go`** is the shared helper library used by every plugin: `GetCommandOutput` (runs the vendor tool with a 10s timeout), `GetRegexpSubmatch` / `GetRegexpAllSubmatch` (parsing), `MarshallJSON`, `TrimSpacesLeftAndRight`. Parsers are regex-based over raw CLI text output.

The two CLI operations map to Zabbix as: **Discovery** (`-d ct|ld|pd`) feeds LLD rules; **Status** (`-s ct,<CT> | ld,<CT>,<LD> | pd,<CT>,<PD>`) feeds items.

## Zabbix integration (`zabbix/`)

- `userparameter_raidstat.conf` — agent UserParameters that call `sudo /opt/raidstat/raidstat`. The configured Zabbix host must define macro `{$RAID_VENDOR}`.
- `raidstat.sudoers` — grants the `zabbix` user passwordless sudo for the binary.
- `zbx_raid_monitoring.xml` — template for the passive agent; `zbx_raid_monitoring_active.yaml` — template for the active agent.

## Conventions

- Errors are handled by printing to stdout and `os.Exit(1)` throughout — there is no error propagation; follow this pattern for consistency.
- Go 1.18, module `github.com/ps78674/zabbix-raidstat`. Only external dep is `ps78674/docopt.go`. `-buildmode=plugin` ties builds to Linux and requires the host and plugins be built with the same Go toolchain/version.
