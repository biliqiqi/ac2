#ac2

[English](README.md) | 中文

一个命令行工具包用于代理Claude Code和Gemini CLI，让其可以通过Web页面来进行交互。同时支持让不同的AI Agents基于MCP互相调用。

## 目录

- [安装](#安装)
  - [下载预编译二进制文件（推荐）](#下载预编译二进制文件推荐)
  - [通过 go install 安装](#通过-go-install-安装)
  - [从源码构建](#从源码构建)
- [使用](#使用)
  - [Web 终端](#web-终端)
  - [MCP 交互](#mcp-交互)

## 安装

### 下载预编译二进制文件（推荐）

为您的平台下载最新版本：

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
- 下载 [ac2-windows-amd64.exe](https://github.com/biliqiqi/ac2/releases/latest/download/ac2-windows-amd64.exe)

或访问 [Releases 页面](https://github.com/biliqiqi/ac2/releases) 查看所有版本和平台。

### 通过 go install 安装

```bash
go install github.com/biliqiqi/ac2/cmd/ac2@latest
```

### 从源码构建

```bash
git clone https://github.com/biliqiqi/ac2.git
cd ac2

# 安装 Web 终端依赖
cd internal/webterm
npm install
cd ../..

# 构建二进制文件
make build
```


## 使用

### Web 终端
启用 Web 终端控制

```
ac2 --entry codex --web-user $USERNAME --web-pass $PASSWORD 
```

网页端将会以 HTTP Basic Auth的方式请求授权，输入刚刚设置的账号和密码即可。

或者也可以使用禁用终端交互，只使用Web段的交互，只需添加 `--no-tui`即可。


### MCP 交互

ac2 支持给 Gemini CLI 和 Claude Code 添加 `stdio` 模式的 MCP 服务器，用于直接基于命令行调用 Gemini CLI、Claude Code 以及 Codex 。

*注：由于 Codex 严格的沙箱机制，暂时不支持让 Codex 调用其他 AI 工具。*


给 Claude Code 添加 MCP 服务器

```
claude mcp add ac2 -- ac2 mcp-stdio
```

给 Gemini CLI 添加 MCP 服务器

```
gemini mcp add ac2 ac2 mcp-stdio
```

如果你的 AI 工具依赖某些环境变量，只需要在添加的时候使用 `--env` 指定环境变量即可。

成功添加 MCP 服务器之后，你可以在 CLI 工具的交互界面中使用 `/ac2:ask-gemini`, `/ac2:ask-claude`, `/ac2:ask-codex` 等命令来与其他 CLI 工具进行交互。

