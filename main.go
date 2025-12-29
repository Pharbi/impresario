package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Config struct {
	Host           string
	Port           string
	User           string
	SSHKey         string
	KnownHostsFile string
}

func getConfig() Config {
	homeDir, _ := os.UserHomeDir()
	defaultKnownHosts := homeDir + "/.impresario/known_hosts"

	return Config{
		Host:           getEnv("IMPRESARIO_HOST", "localhost"),
		Port:           getEnv("IMPRESARIO_PORT", "22"),
		User:           getEnv("IMPRESARIO_USER", "ubuntu"),
		SSHKey:         getEnv("IMPRESARIO_KEY", ""),
		KnownHostsFile: getEnv("IMPRESARIO_KNOWN_HOSTS", defaultKnownHosts),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func safelyShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func sshExec(cfg Config, command string) error {
	knownHostsDir := cfg.KnownHostsFile[:strings.LastIndex(cfg.KnownHostsFile, "/")]
	if err := os.MkdirAll(knownHostsDir, 0700); err != nil {
		return fmt.Errorf("failed to create known_hosts directory: %w", err)
	}

	args := []string{
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", cfg.KnownHostsFile),
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		"-p", cfg.Port,
	}

	if cfg.SSHKey != "" {
		args = append(args, "-i", cfg.SSHKey)
	}

	args = append(args, fmt.Sprintf("%s@%s", cfg.User, cfg.Host), command)

	cmd := exec.Command("ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func printHelp() {
	fmt.Println(`
Impresario - Remote execution for AI agents

Usage:
  impresario <command> [arguments]

Commands:
  exec <command>       Execute command on remote server
  read <path>          Read a file from remote
  write <path> <text>  Write content to a file
  ls [path]            List directory (default: ~)
  info                 Show connection info
  test                 Test SSH connection
  help                 Show this help

Environment:
  IMPRESARIO_HOST        Remote host (default: localhost)
  IMPRESARIO_PORT        SSH port (default: 22)
  IMPRESARIO_USER        SSH user (default: ubuntu)
  IMPRESARIO_KEY         Path to SSH key (optional if using ssh-agent)
  IMPRESARIO_KNOWN_HOSTS Path to known_hosts file (default: ~/.impresario/known_hosts)

Examples:
  impresario exec "git clone https://github.com/user/repo"
  impresario exec "cd repo && npm install && npm test"
  impresario read ~/projects/repo/README.md
  impresario write ~/test.py "print('hello world')"
  impresario ls ~/projects

Works with any SSH server. No special software required on remote.
`)
}

func main() {
	cfg := getConfig()

	if port, err := strconv.Atoi(cfg.Port); err != nil || port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "Invalid port: %s (must be 1-65535)\n", cfg.Port)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error

	switch cmd {
	case "exec", "run":
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Usage: impresario exec <command>")
			os.Exit(1)
		}
		err = sshExec(cfg, strings.Join(args, " "))

	case "read", "cat":
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Usage: impresario read <path>")
			os.Exit(1)
		}
		err = sshExec(cfg, fmt.Sprintf("cat %s", safelyShellQuote(args[0])))

	case "write":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: impresario write <path> <content>")
			os.Exit(1)
		}
		content := strings.Join(args[1:], " ")
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		err = sshExec(cfg, fmt.Sprintf("echo %s | base64 -d > %s", safelyShellQuote(encoded), safelyShellQuote(args[0])))
		if err == nil {
			fmt.Printf("Written to %s\n", args[0])
		}

	case "ls":
		path := "$HOME"
		if len(args) > 0 {
			path = safelyShellQuote(args[0])
		}
		err = sshExec(cfg, fmt.Sprintf("ls -la %s", path))

	case "info":
		fmt.Printf(`Impresario Remote Connection:
  Host: %s
  Port: %s
  User: %s
  Key:  %s
`, cfg.Host, cfg.Port, cfg.User, orDefault(cfg.SSHKey, "(ssh-agent)"))

	case "test":
		fmt.Printf("Testing connection to %s@%s:%s...\n", cfg.User, cfg.Host, cfg.Port)
		err = sshExec(cfg, "echo 'Connection successful!' && uname -a")
		if err == nil {
			fmt.Println("\n✓ Connection test passed")
		}

	case "help", "-h", "--help":
		printHelp()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printHelp()
		os.Exit(1)
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
