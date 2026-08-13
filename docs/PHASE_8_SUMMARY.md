# Phase 8: Plugin System Implementation

**Status:** Complete ✅  
**Release:** v0.2.0 (alongside Phase 7 MCP integration)  
**Scope:** Local-path plugin installation with dynamic Cobra subcommand registration

## Overview

Phase 8 completes Lana's core feature set by implementing a general-purpose plugin system. Plugins are installable CLI command extensions that become new `lana <name>` subcommands. This gives users the ability to extend Lana without modifying its codebase — the same extensibility model as `gh extension`, Docker CLI plugins, or npm scripts.

The design emphasizes:
- **Simplicity over scope:** Local-path install only (no GitHub/package-manager discovery in v0.2)
- **Safety without sandboxing:** Plugins run with user permissions; no execution constraints beyond workspace policy
- **Frictionless integration:** Installed plugins become immediate subcommands with zero configuration
- **Optional MCP bridging:** Plugins can declare MCP servers, unifying two extension mechanisms

## Architecture

### Core Package: `internal/plugin/`

**`manifest.go`** — Plugin metadata and validation
```go
type Manifest struct {
    Name        string                   // ^[a-z][a-z0-9-]*$ pattern (safe as subcommand name)
    Version     string                   // Semantic version (1.0.0)
    Description string                   // Short description for CLI listing
    Entrypoint  string                   // Relative path to executable (e.g., run.sh)
    MCPServers  []config.MCPServerConfig // Optional: MCP servers provided by this plugin
}
```

- `LoadManifest(dir string) (*Manifest, error)` — Parse `manifest.yaml` from plugin directory
- `Validate(workDir string) error` — Verify name pattern, entrypoint exists and is executable
- `EntrypointPath(pluginDir string) string` — Return absolute path to entrypoint

**`registry.go`** — Plugin discovery with partial-failure tolerance
- `InstalledPlugins(pluginsDir string) ([]*Manifest, error)` — Scan `~/.lana/plugins/` subdirectories
  - Creates `pluginsDir` if it doesn't exist
  - Skips and logs invalid plugins (one broken plugin doesn't block others)
  - Returns all valid manifests found
- `FindPlugin(pluginsDir, name string) (*Manifest, error)` — Locate a single plugin by name

**`install.go`** — Plugin installation with collision detection
- `Install(sourcePath, pluginsDir string, reserved map[string]bool) (*Manifest, error)`
  - Validates manifest and entrypoint at source path
  - Rejects if name collides with reserved built-in command names
  - Rejects if plugin with that name already exists in pluginsDir
  - Copies entire plugin directory tree to `pluginsDir/<name>/`
  - `chmod +x` the entrypoint
  - Cleans up on failure via `os.RemoveAll`

**`remove.go`** — Plugin uninstallation
- `Remove(pluginsDir, name string) error` — Delete plugin directory after existence check

**`exec.go`** — Plugin subprocess execution
- `Run(ctx context.Context, pluginDir string, m *Manifest, args []string, stdin io.Reader, stdout, stderr io.Writer) error`
  - Builds `exec.Cmd` for manifest's resolved entrypoint path
  - Wires streams directly (full passthrough, not captured)
  - **Critically:** Sets `cmd.Dir` to **caller's working directory**, not plugin install directory
  - Respects context cancellation

### CLI Layer: `internal/cmd/plugin.go`

Follows the exact Cobra pattern established in Phase 7's `internal/cmd/mcp.go`:

**`lana plugin list`** — Enumerate installed plugins
```
Installed Plugins:

1. my-plugin (1.0.0)
   A useful plugin
   Entrypoint: run.sh
```

**`lana plugin install <path>`** — Validate and copy
- Validates manifest and entrypoint at source path
- Checks reserved command names (chat, run, version, config, providers, models, sessions, doctor, mcp, plugin)
- Checks for collisions with already-installed plugins
- Copies plugin directory to `~/.lana/plugins/<name>/`
- **If plugin declares MCPServers:** Registers them in global config with absolute command paths

**`lana plugin remove <name>`** — Uninstall
- Deletes plugin directory
- Note: User must manually remove MCP servers if plugin provided them (document this explicitly)

**`lana plugin info <name>`** — Show details
```
Plugin: my-plugin
Version: 1.0.0
Description: A useful plugin
Entrypoint: run.sh

MCP Servers: 1
  1. my-server (stdio)
```

### Dynamic Subcommand Registration: `internal/cmd/root.go`

Modified `Execute()` to register plugins before Cobra's command tree resolution:

```go
func Execute() error {
    registerInstalledPlugins()  // Non-fatal; failures log to stderr
    return rootCmd.Execute()
}

func registerInstalledPlugins() {
    pluginsDir := defaultPluginsPath()
    plugins, err := plugin.InstalledPlugins(pluginsDir)
    if err != nil { return }  // Skip on dir scan errors
    
    for _, p := range plugins {
        cmd := &cobra.Command{
            Use:               p.Name,
            Short:             p.Description,
            DisableFlagParsing: true,  // Plugin handles its own flags
            RunE: func(cmd *cobra.Command, args []string) error {
                ctx := context.Background()
                return plugin.Run(ctx, pluginDir, p, args, os.Stdin, os.Stdout, os.Stderr)
            },
        }
        rootCmd.AddCommand(cmd)
    }
}
```

**Key design point:** `DisableFlagParsing: true` prevents Cobra from interpreting the plugin's flags as Lana flags. The plugin receives raw arguments and full control.

## Design Decisions

### 1. Local-Path Install Only (v0.2)

**Decision:** GitHub-based discovery and `gh release download` style installation deferred to v0.3.

**Rationale:**
- Requires GitHub API client (not present in Lana today)
- Must handle repo search, release-asset resolution by OS/arch, rate limits
- Roughly doubles implementation scope
- User can still `git clone` a repo and `lana plugin install ./that-repo`

### 2. No Sandboxing

**Decision:** Plugins run with full user permissions; no execution constraints beyond workspace policy.

**Rationale:**
- Plugin execution is a direct, deliberate user action ("type `lana myplugin` at shell")
- Different trust model from MCP tool calls (which an agent initiates autonomously)
- Trying to sandbox subprocess execution (chroot, seccomp, containers) is fragile and footgun-prone
- **Documentation must plainly state this is equivalent to running any installed CLI tool**

### 3. Working Directory: Caller's, Not Plugin's

**Decision:** `cmd.Dir = os.Getwd()` → plugins run relative to where the user typed the command

**Rationale:**
- Matches user expectation: `lana myplugin analyze file.go` looks for `./file.go` relative to cwd
- Plugins can assume they're tools (like `grep`, `awk`, etc.), not special environments
- Consistent with how Docker CLI plugins, gh extensions, npm scripts work
- If a plugin needs to reference files in its install directory, it gets `$LANA_PLUGIN_DIR` env var? (Not implemented in Phase 8, but easy to add)

### 4. Entrypoint as Relative Path

**Decision:** `Entrypoint` in `manifest.yaml` is a relative path; resolved against the plugin directory at runtime

**Rationale:**
- Plugin author specifies `entrypoint: bin/run.sh` or `entrypoint: run.py`
- Lana resolves it to absolute path `~/.lana/plugins/my-plugin/bin/run.sh` before exec
- Avoids ambiguity (is `/usr/bin/run.sh` in the plugin dir or the system?)

### 5. Manifest Name Validation: `^[a-z][a-z0-9-]*$`

**Decision:** Strict pattern enforced at validation time.

**Rationale:**
- Uppercase letters confuse subcommand names (Cobra lowercases them anyway)
- Leading digits are ambiguous in shell completion
- Hyphens are idiomatic for CLI tool names
- Pattern is safe to use as a directory name and command name

### 6. Collision Detection Against Reserved Names

**Decision:** Hardcoded set of reserved built-in command names: `chat, run, version, config, providers, models, sessions, doctor, mcp, plugin`.

**Rationale:**
- Small, explicit, and direct
- Matches codebase preference: reflect over these names, don't generate them
- Easy to update when new commands are added (just edit the set in `plugin.go`)
- User gets a clear error message: "conflicts with a built-in Lana command"

### 7. Partial-Failure Tolerance in Registry

**Decision:** `InstalledPlugins()` skips invalid plugins with stderr warnings, doesn't abort.

**Rationale:**
- Matches `internal/mcp/manager.go:66-92` MCP manager startup pattern
- One broken plugin (e.g., missing entrypoint file) shouldn't block all others
- Lana still starts, user sees warnings, can then fix or remove the broken plugin

### 8. MCP Server Registration via Install

**Decision:** If plugin's manifest declares `MCPServers`, `lana plugin install` automatically registers them in global config.

**Rationale:**
- One install operation does everything the user expects
- MCP server commands are rewritten to absolute paths so they work from any directory
- Mirrors `lana mcp add` workflow but automated for plugin authors
- User must manually remove MCP servers on uninstall (not automated, documented in USER_GUIDE)

### 9. No Plugin-to-Plugin Dependency Management

**Decision:** Plugins can't declare dependencies on other plugins; each is standalone.

**Rationale:**
- Simplifies v0.2 scope
- User can manually install prerequisites if needed
- Future phases can add plugin manifest `requires: [other-plugin]` if demand arises

## Testing

### Unit Tests: `internal/plugin/*_test.go`

**`manifest_test.go`** — 8 test cases
- Valid manifest with all fields
- Missing name (error)
- Invalid name patterns (uppercase, leading digits, special chars)
- Valid names with hyphens
- Missing entrypoint (error)
- Entrypoint not found (error)
- Entrypoint not executable (error)
- Entrypoint is a directory (error)

**`install_test.go`** — 3 test cases
- Successful install (verifies file copy and chmod)
- Reserved name collision (chat, run, etc.)
- Already-installed collision

**`registry_test.go`** — 3 test cases
- Scan with mix of valid and broken plugins (partial-failure tolerance)
- Auto-create plugins dir if missing
- Find plugin by name (found and not-found cases)

**`exec_test.go`** — 3 test cases
- Plugin execution with args (output passthrough)
- Context cancellation
- Non-existent entrypoint (error)

**`remove_test.go`** — 2 test cases
- Successful removal
- Plugin not found (error)

### Test Fixture: `internal/plugin/testdata/fixture/echoplugin.sh`

Simple bash script that echoes arguments and exits cleanly — used by `exec_test.go` to verify arg and output passthrough.

### Manual End-to-End Testing

```bash
# Create test plugin
mkdir /tmp/test-plugin
cat > /tmp/test-plugin/manifest.yaml << 'EOF'
name: testplugin
version: 1.0.0
description: A test plugin
entrypoint: run.sh
EOF

cat > /tmp/test-plugin/run.sh << 'EOF'
#!/bin/bash
echo "Test plugin called with args: $@"
echo "Working directory: $(pwd)"
EOF

chmod +x /tmp/test-plugin/run.sh

# Test install
./lana plugin install /tmp/test-plugin
# Output: Installed plugin 'testplugin' (v1.0.0)

# Test list
./lana plugin list
# Output: Shows testplugin with description and entrypoint

# Test info
./lana plugin info testplugin
# Output: Full manifest details

# Test dynamic subcommand registration
./lana testplugin hello world
# Output: Test plugin called with args: hello world
#         Working directory: /home/deagy/sdk/lana

# Test collision detection
mkdir /tmp/bad-plugin
echo "name: chat" > /tmp/bad-plugin/manifest.yaml
./lana plugin install /tmp/bad-plugin 2>&1 | grep "conflicts with"
# Output: ...conflicts with a built-in Lana command

# Test remove
./lana plugin remove testplugin
# Output: Removed plugin 'testplugin'

./lana plugin list
# Output: No plugins installed.
```

All passed.

## Files Changed/Created

### New Package
- `internal/plugin/manifest.go` — 65 lines
- `internal/plugin/manifest_test.go` — 172 lines
- `internal/plugin/registry.go` — 52 lines
- `internal/plugin/registry_test.go` — 107 lines
- `internal/plugin/install.go` — 68 lines
- `internal/plugin/install_test.go` — 124 lines
- `internal/plugin/remove.go` — 27 lines
- `internal/plugin/remove_test.go` — 50 lines
- `internal/plugin/exec.go` — 33 lines
- `internal/plugin/exec_test.go` — 74 lines
- `internal/plugin/testdata/fixture/echoplugin.sh` — 5 lines

### CLI Layer
- `internal/cmd/plugin.go` — 184 lines (Cobra command definitions + helpers)

### Root Command
- `internal/cmd/root.go` — Modified Execute() + registerInstalledPlugins() + defaultPluginsPath()

### Documentation
- `docs/USER_GUIDE.md` — Added "Writing a Plugin" section (120 lines)
- `README.md` — Updated status, added Plugins section, removed plugin system from Limitations
- `CHANGELOG.md` — Added Phase 8 features and status update
- `docs/PHASE_8_SUMMARY.md` — This file (comprehensive implementation guide)

## Integration Points

### With Phase 7 MCP System

- Plugin's `manifest.yaml` can declare `MCPServers`
- `lana plugin install` automatically registers declared MCP servers
- MCP server command paths are rewritten to absolute paths inside plugin directory
- Enables use case: "one `plugin install` delivers both a subcommand and tools to the agent"

### With Configuration System

- Plugins directory: `~/.lana/plugins/` (determined by `defaultPluginsPath()`)
- MCP server config merged into global `~/.lana/config.yaml` on plugin install
- User can manually edit config to disable/remove MCP servers if desired

### With Workspace Policy

- `internal/policy/` not involved in plugin execution (plugins run with full user permissions)
- Plugin subprocess respects context cancellation (used by CLI timeout mechanisms)

## Known Limitations

1. **No GitHub-based discovery** — Can't do `lana plugin install user/repo`; must clone and install locally
2. **No plugin dependency management** — Can't declare `requires: [other-plugin]`
3. **Manual MCP cleanup** — Uninstalling a plugin doesn't remove its MCP servers; user must manually `lana mcp remove`
4. **No plugin marketplace** — No central registry; plugins are standalone directories
5. **No version checking** — Installing a newer version of a plugin just overwrites; no version pinning or rollback
6. **No plugin isolation** — Plugins run with full user permissions; no sandboxing

## Future Work (v0.3+)

- **GitHub plugin discovery** — `lana plugin install deagy/my-plugin` pulls from GitHub releases
- **Plugin marketplace** — Centralized registry of community plugins
- **Dependency management** — Plugins can declare and enforce prerequisites
- **Plugin templates** — `lana plugin create --language bash my-plugin` scaffolds a starter structure
- **Plugin auto-update** — Check for newer versions, upgrade with `--update` flag
- **Plugin environment variables** — Pass `$LANA_PLUGIN_DIR`, `$LANA_HOME` to plugins
- **Plugin lifecycle hooks** — `pre-install`, `post-install`, `pre-remove` scripts

## Verification Checklist

- ✅ All unit tests pass (`go test ./internal/plugin/...`)
- ✅ Code builds without errors (`go build ./cmd/lana`)
- ✅ `lana plugin list` works (shows no plugins initially)
- ✅ `lana plugin install <path>` validates and copies correctly
- ✅ `lana plugin info <name>` displays correct manifest fields
- ✅ Installed plugin runs as subcommand (`lana <name> args...`)
- ✅ Plugin receives correct working directory and arguments
- ✅ Collision detection rejects reserved names
- ✅ Collision detection rejects already-installed plugins
- ✅ `lana plugin remove <name>` deletes plugin directory
- ✅ Partial-failure tolerance: invalid plugins skip, others still load
- ✅ MCP server registration works (plugin with MCPServers shows in `lana mcp list`)

## Conclusion

Phase 8 completes Lana's extensibility story. Combined with Phase 7's MCP protocol support, Lana now offers two complementary extension mechanisms:
- **MCP servers** (Phase 7) — Extend the agent's tool set
- **CLI plugins** (Phase 8) — Extend Lana's command set

This achieves the stated v1.0 feature set goal. Remaining roadmap items (v0.3, v1.0) are nice-to-haves (web UI, multi-agent, GitHub plugin discovery) but not blockers for production use.
