# Changelog

All notable changes to Lana will be documented in this file.

## [Unreleased]

### Features
- (Planned features go here)

### Improvements
- (Improvements go here)

### Bug Fixes
- (Bug fixes go here)

## [0.1.0] - 2025-08-12

### Features
- Interactive chat mode (`lana chat`)
- Non-interactive execution (`lana run`)
- Structured output formats (JSON, JSONL, plain text)
- 9 production tools (file ops, shell, git, search)
- Session management with persistence
- Safety framework with approval policies
- Exit codes for automation
- Multiple provider support (OpenAI, Ollama)

### Tools
- `read_file` — Read file contents
- `write_file` — Write/create files
- `list_files` — Directory listing
- `exec` — Shell command execution
- `git_status` — Git status
- `git_diff` — Git diff
- `git_commit` — Git commits
- `git_branch` — Branch management
- `search` — File search with ripgrep/grep

### Commands
- `chat` — Interactive conversations
- `run` — Non-interactive execution
- `sessions` — Session management
- `providers` — Provider management
- `models` — Model listing
- `config` — Configuration management
- `doctor` — Diagnostics
- `version` — Version info

### Approval Modes
- `ask` — Prompt for each operation
- `auto-edit` — Auto-approve edits
- `full-auto` — Auto-approve all

### Documentation
- USER_GUIDE.md — Comprehensive user guide
- API.md — Developer API reference
- RELEASE_CHECKLIST.md — Release procedures
- CLAUDE.md — Project guidelines

### Tests
- 30 tests with 100% pass rate
- Coverage for all core infrastructure
- File operations tests
- Policy/safety tests
- Output formatter tests

## Version Scheme

This project follows [Semantic Versioning](https://semver.org/).

### Release History

| Version | Date | Type | Notes |
|---------|------|------|-------|
| 0.1.0 | 2025-08-12 | Initial | First release with core features |

## Upgrade Guide

### From v0.0.x to v0.1.0

No breaking changes. New features:

1. `lana run` command for non-interactive use
2. JSON/JSONL output formats
3. Multiple provider support
4. Session management

All existing functionality preserved.

## Known Issues

None reported yet.

## Future Roadmap

### v0.2.0 (Q3 2025)
- [ ] MCP protocol integration
- [ ] Plugin system
- [ ] Advanced output modes (CSV, Markdown)
- [ ] Performance optimization

### v0.3.0 (Q4 2025)
- [ ] Web UI dashboard
- [ ] Multi-agent orchestration
- [ ] Integration tests
- [ ] Production deployment guide

### v1.0.0 (2026)
- [ ] Stable API
- [ ] Enterprise features
- [ ] Commercial support

## Contributing

See [CONTRIBUTING.md](docs/CONTRIBUTING.md) for guidelines.

## License

See [LICENSE](LICENSE) file.

## Support

- Issues: https://github.com/deagy/lana/issues
- Discussions: https://github.com/deagy/lana/discussions
- Email: support@example.com

---

**Format**: This changelog follows [Keep a Changelog](https://keepachangelog.com/).
