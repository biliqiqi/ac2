# ac2

English | [中文](README.zh.md)

A command-line toolkit for proxying Claude Code and Gemini CLI, enabling web-based interaction. Also supports MCP-based cross-calling between different AI agents.

## Table of Contents

- [Installation](#installation)
  - [Download Precompiled Binaries (Recommended)](#download-precompiled-binaries-recommended)
  - [Install via go install](#install-via-go-install)
  - [Build from Source](#build-from-source)
- [Usage](#usage)
  - [Web Terminal](#web-terminal)
  - [MCP Integration](#mcp-integration)

## Installation

### Download Precompiled Binaries (Recommended)

Download the latest version for your platform:

**Linux (amd64)**
```bash
wget https://github.com/biliqiqi/ac2/releases/latest/download/ac2-linux-amd64
chmod +x ac2-linux-amd64
sudo mv ac2-linux-amd64 /usr/local/bin/ac2
```

**Linux (arm64)**
```bash
wget https://github.com/biliqiqi/ac2/releases/latest/download/ac2-linux-arm64
chmod +x ac2-linux-arm64
sudo mv ac2-linux-arm64 /usr/local/bin/ac2
```

**macOS (Intel)**
```bash
wget https://github.com/biliqiqi/ac2/releases/latest/download/ac2-darwin-amd64
chmod +x ac2-darwin-amd64
mv ac2-darwin-amd64 ac2
```

**macOS (Apple Silicon)**
```bash
wget https://github.com/biliqiqi/ac2/releases/latest/download/ac2-darwin-arm64
chmod +x ac2-darwin-arm64
mv ac2-darwin-arm64 ac2
```

**Windows**
- Download [ac2-windows-amd64.exe](https://github.com/biliqiqi/ac2/releases/latest/download/ac2-windows-amd64.exe)

Or visit the [Releases page](https://github.com/biliqiqi/ac2/releases) to see all versions and platforms.

### Install via go install

```bash
go install github.com/biliqiqi/ac2/cmd/ac2@latest
```

### Build from Source

```bash
git clone https://github.com/biliqiqi/ac2.git
cd ac2

# Install web terminal dependencies
cd internal/webterm
npm install
cd ../..

# Build binary
make build
```


## Usage

### Web Terminal

Enable web terminal control:

```bash
ac2 --entry codex --web-user $USERNAME --web-pass $PASSWORD
```

The web interface will request authorization via HTTP Basic Auth. Enter the username and password you just set.

Alternatively, you can disable terminal interaction and use only the web interface by adding the `--no-tui` flag.


### MCP Integration

ac2 supports adding `stdio` mode MCP servers to Gemini CLI and Claude Code, enabling command-line based calls to Gemini CLI, Claude Code, and Codex.

*Note: Due to Codex's strict sandbox mechanism, calling other AI tools from Codex is currently not supported.*


Add MCP server to Claude Code:

```bash
claude mcp add ac2 -- ac2 mcp-stdio
```

Add MCP server to Gemini CLI:

```bash
gemini mcp add ac2 ac2 mcp-stdio
```

If your AI tool depends on certain environment variables, simply specify them using the `--env` flag when adding the server.

Once the MCP server is successfully added, you can use commands like `/ac2:ask-gemini`, `/ac2:ask-claude`, `/ac2:ask-codex` in the CLI tool's interactive interface to interact with other CLI tools.
