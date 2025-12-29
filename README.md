# Impresario

Remote execution CLI for AI agents. Let any AI coding agent work on any remote server over SSH.

**Works with any SSH server you control.** A managed sandbox service with batteries-included environments is coming soon.

## Installation

### Download Binary

```bash
# macOS (Apple Silicon)
curl -L https://github.com/Pharbi/impresario/releases/latest/download/impresario-darwin-arm64 -o /usr/local/bin/impresario
chmod +x /usr/local/bin/impresario

# macOS (Intel)
curl -L https://github.com/Pharbi/impresario/releases/latest/download/impresario-darwin-amd64 -o /usr/local/bin/impresario
chmod +x /usr/local/bin/impresario

# Linux
curl -L https://github.com/Pharbi/impresario/releases/latest/download/impresario-linux-amd64 -o /usr/local/bin/impresario
chmod +x /usr/local/bin/impresario
```

### Build from Source

```bash
cd impresario
make build
make install
```

## Configuration

```bash
export IMPRESARIO_HOST=your-server.com
export IMPRESARIO_PORT=22
export IMPRESARIO_USER=ubuntu
export IMPRESARIO_KEY=~/.ssh/id_ed25519  # optional if using ssh-agent
export IMPRESARIO_KNOWN_HOSTS=~/.impresario/known_hosts  # optional, this is the default
```

Impresario uses a dedicated `known_hosts` file (`~/.impresario/known_hosts`) to avoid polluting your main SSH configuration. This is designed for ephemeral/managed VPS instances where IPs may be recycled.

## Usage

```bash
# Execute commands
impresario exec "git clone https://github.com/user/repo"
impresario exec "cd repo && npm install && npm test"

# File operations
impresario read ~/projects/file.js
impresario write ~/test.py "print('hello')"
impresario ls ~/projects

# Connection
impresario info
impresario test
```

## Works with Any Server

Impresario works with any SSH-accessible server:

```bash
# Your VPS
export IMPRESARIO_HOST=my-vps.digitalocean.com
impresario exec "whoami"

# EC2 instance
export IMPRESARIO_HOST=ec2-12-34-56-78.compute-1.amazonaws.com
export IMPRESARIO_USER=ec2-user
impresario exec "docker ps"

# Raspberry Pi
export IMPRESARIO_HOST=192.168.1.100
export IMPRESARIO_USER=pi
impresario exec "python3 script.py"
```

**Requirements:** SSH server running, SSH key authentication configured. That's it.

## Use with AI Agents

Any agent that can run bash can use Impresario:

**Aider:**
```bash
aider --message "Use 'impresario exec <cmd>' to run commands on the remote server"
```

**Goose:**
```bash
goose "run 'impresario exec git status' on the remote"
```

**Codex/Gemini:**
```bash
codex "use 'impresario exec npm test' to run tests on the remote server"
```

**Claude Code:** Use the MCP server ([mcp-impresario](https://github.com/Pharbi/mcp-impresario)) for native integration.

## License

MIT
