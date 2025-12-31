package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const (
	SecretsPath = "/run/secrets/env"
)

var CommonSecrets = []string{
	"ANTHROPIC_API_KEY",
	"OPENAI_API_KEY",
	"GOOGLE_API_KEY",
	"GITHUB_TOKEN",
}

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

// sshExecOutput executes a command and returns its output (for reading secrets)
func sshExecOutput(cfg Config, command string) (string, error) {
	knownHostsDir := cfg.KnownHostsFile[:strings.LastIndex(cfg.KnownHostsFile, "/")]
	if err := os.MkdirAll(knownHostsDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create known_hosts directory: %w", err)
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
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// promptSecret reads a password-style input (hidden)
func promptSecret(key string) (string, error) {
	fmt.Printf("  %s: ", key)

	// Check if stdin is a terminal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after hidden input
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}

	// Fallback for non-terminal (piped input)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// readExistingSecrets reads current secrets from remote
func readExistingSecrets(cfg Config) (map[string]string, error) {
	output, err := sshExecOutput(cfg, fmt.Sprintf("cat %s 2>/dev/null || echo ''", SecretsPath))
	if err != nil {
		return nil, err
	}

	secrets := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "export ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "export "), "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := strings.Trim(parts[1], "\"")
				secrets[key] = value
			}
		}
	}
	return secrets, nil
}

// writeSecrets writes all secrets to remote tmpfs
func writeSecrets(cfg Config, secrets map[string]string) error {
	var lines []string
	for key, value := range secrets {
		// Escape double quotes in value
		escaped := strings.ReplaceAll(value, "\"", "\\\"")
		lines = append(lines, fmt.Sprintf("export %s=\"%s\"", key, escaped))
	}

	content := strings.Join(lines, "\n") + "\n"

	// Write via SSH using heredoc to handle special characters
	cmd := fmt.Sprintf("cat > %s << 'SECRETS_EOF'\n%sSECRETS_EOF", SecretsPath, content)
	if err := sshExec(cfg, cmd); err != nil {
		return fmt.Errorf("failed to write secrets: %w", err)
	}

	// Set permissions
	if err := sshExec(cfg, fmt.Sprintf("chmod 600 %s", SecretsPath)); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

// secretsSetInteractive prompts for all common secrets
func secretsSetInteractive(cfg Config) error {
	fmt.Println("Enter API keys (leave blank to skip):")
	fmt.Println()

	secrets := make(map[string]string)

	for _, key := range CommonSecrets {
		value, err := promptSecret(key)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", key, err)
		}
		if value != "" {
			secrets[key] = value
		}
	}

	if len(secrets) == 0 {
		fmt.Println("No secrets provided.")
		return nil
	}

	if err := writeSecrets(cfg, secrets); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("✓ %d secret(s) written to remote (RAM only)\n", len(secrets))
	fmt.Println("  Start a new shell or run: source /run/secrets/env")
	return nil
}

// secretsSetOne prompts for a single secret
func secretsSetOne(cfg Config, key string) error {
	value, err := promptSecret(key)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", key, err)
	}

	if value == "" {
		fmt.Printf("No value provided for %s\n", key)
		return nil
	}

	// Read existing secrets, update the one key, write back
	existing, err := readExistingSecrets(cfg)
	if err != nil {
		existing = make(map[string]string)
	}
	existing[key] = value

	if err := writeSecrets(cfg, existing); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("✓ Secret %s written to remote (RAM only)\n", key)
	fmt.Println("  Start a new shell or run: source /run/secrets/env")
	return nil
}

// secretsShow displays which secrets are configured (not their values)
func secretsShow(cfg Config) error {
	output, err := sshExecOutput(cfg, fmt.Sprintf("cat %s 2>/dev/null || echo ''", SecretsPath))
	if err != nil {
		return err
	}

	if strings.TrimSpace(output) == "" {
		fmt.Println("No secrets configured.")
		fmt.Println("Run 'impresario secrets set' to add them.")
		return nil
	}

	fmt.Println("Configured secrets:")
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "export ") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], "export ")
				value := strings.Trim(parts[1], "\"")
				if value != "" {
					fmt.Printf("  ✓ %s (set)\n", key)
				} else {
					fmt.Printf("  ✗ %s (empty)\n", key)
				}
			}
		}
	}
	return nil
}

// secretsClear removes all secrets from remote
func secretsClear(cfg Config) error {
	if err := sshExec(cfg, fmt.Sprintf("> %s", SecretsPath)); err != nil {
		return err
	}
	fmt.Println("Secrets cleared.")
	return nil
}

func printSecretsHelp() {
	fmt.Println(`
Manage API keys on remote sandbox

Keys are written directly to tmpfs (RAM) on the remote server.
They never touch the platform's servers or your local disk.

Usage:
  impresario secrets <subcommand>

Subcommands:
  set [KEY]    Set API keys on remote
               Without arguments, prompts for all common keys (Anthropic, OpenAI, etc.)
               With a KEY argument, prompts for just that key.
  show         Show which keys are configured (not their values)
  clear        Remove all secrets from remote

Examples:
  impresario secrets set                    # Prompt for all common keys
  impresario secrets set ANTHROPIC_API_KEY  # Set a specific key
  impresario secrets set MY_CUSTOM_KEY      # Set any custom key
  impresario secrets show                   # Check what's configured
  impresario secrets clear                  # Clear all secrets
`)
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
  secrets <subcmd>     Manage API keys on remote sandbox
  help                 Show this help

Secrets Subcommands:
  secrets set [KEY]    Set API keys (interactive, never stored locally)
  secrets show         Show which keys are configured
  secrets clear        Clear all secrets from remote

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
  impresario secrets set                    # Set all common API keys
  impresario secrets set ANTHROPIC_API_KEY  # Set a specific key
  impresario secrets show                   # Check configured secrets

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

	case "secrets":
		if len(args) == 0 {
			printSecretsHelp()
			os.Exit(0)
		}
		subcmd := args[0]
		subargs := args[1:]

		switch subcmd {
		case "set":
			if len(subargs) == 0 {
				err = secretsSetInteractive(cfg)
			} else {
				err = secretsSetOne(cfg, subargs[0])
			}
		case "show":
			err = secretsShow(cfg)
		case "clear":
			err = secretsClear(cfg)
		case "help", "-h", "--help":
			printSecretsHelp()
		default:
			fmt.Fprintf(os.Stderr, "Unknown secrets subcommand: %s\n", subcmd)
			printSecretsHelp()
			os.Exit(1)
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
