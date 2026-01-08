# Relocate

A Terminal User Interface (TUI) for quickly SSH-ing into AWS EC2 instances.

## Features

- **Interactive browser**: Visual interface for browsing EC2 instances
- **Real-time search**: Filter instances by name, ID, IP, or type
- **Environment switching**: Toggle between staging and production environments
- **Confirmation dialog**: Prevents accidental connections
- **Responsive UI**: Adapts to terminal size

## Installation

### Quick install (with config setup)

```bash
curl -sSL https://raw.githubusercontent.com/ghazimuharam/relocate/master/install.sh | bash
```

Or manually:

```bash
./install.sh
```

### Using go install

```bash
go install github.com/ghazimuharam/relocate/cmd/relocate@latest
```

### Build from source

```bash
go build -o relocate ./cmd/relocate
```

Optionally install to PATH:

```bash
sudo mv relocate /usr/local/bin/
```

## Setup

### 1. Configure SSH Keys

Create a configuration file at `~/.relocate/config.json`:

```bash
mkdir -p ~/.relocate
cp config.example.json ~/.relocate/config.json
```

Edit `~/.relocate/config.json` with your settings:

```json
{
  "ssh_keys": {
    "staging": "your-staging-key-name",
    "prod": "your-prod-key-name"
  },
  "defaults": {
    "aws_profile": "default",
    "aws_region": "ap-southeast-1",
    "ssh_user": "ubuntu"
  }
}
```

**Required:** `ssh_keys.staging` and `ssh_keys.prod` must be set.

### 2. AWS Credentials

Ensure AWS credentials are configured:

```bash
aws configure
```

Or use environment variables / AWS profiles.

### 3. SSH Keys

Place your SSH private keys in `~/.ssh/` with the filenames you specified in the config.

## Usage

```bash
# Basic usage
./relocate

# With specific AWS profile and region
./relocate --profile staging --region us-west-2

# Filter by tag
./relocate --filter Environment=staging

# Specify SSH user
./relocate --user ec2-user
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Connect to selected instance |
| `Tab` | Toggle between staging/prod |
| `1` | Switch to staging |
| `2` | Switch to production |
| `Esc` | Clear search (or quit) |
| `Ctrl+C` | Quit immediately |
| `Y` / `N` | Confirm/cancel connection |

## Configuration

The configuration file `~/.relocate/config.json` supports:

| Option | Required | Description |
|--------|----------|-------------|
| `ssh_keys.staging` | Yes | SSH key filename for staging |
| `ssh_keys.prod` | Yes | SSH key filename for production |
| `defaults.aws_profile` | No | Default AWS profile |
| `defaults.aws_region` | No | Default AWS region |
| `defaults.ssh_user` | No | Default SSH username |

CLI flags override config defaults.

## CLI Flags

| Flag | Alias | Default | Description |
|------|-------|---------|-------------|
| `--profile` | `-p` | (from config) | AWS profile name |
| `--region` | `-r` | (from config) | AWS region |
| `--filter` | `-f` | - | Tag filter (e.g., `Environment=staging`) |
| `--user` | `-u` | `ubuntu` | SSH username |

## Troubleshooting

### Config file not found

```
Error loading config: config file not found: /Users/you/.relocate/config.json
```

**Solution:**
```bash
mkdir -p ~/.relocate
cp config.example.json ~/.relocate/config.json
# Edit the file with your SSH key names
```

### Invalid config

```
Config validation failed: config file is invalid: staging SSH key not configured
```

**Solution:** Make sure `ssh_keys.staging` and `ssh_keys.prod` are set in your config.

### Failed to load AWS config

**Solution:** Ensure AWS credentials are configured:
```bash
aws configure
```

### No instances found

**Solution:**
- Check your AWS region with `--region`
- Verify your AWS permissions allow EC2 `DescribeInstances`
- Try the `--filter` flag to narrow results

## Requirements

- Go 1.21 or later
- AWS credentials configured
- SSH keys in `~/.ssh/`
- Network access to target EC2 instances

## License

MIT
