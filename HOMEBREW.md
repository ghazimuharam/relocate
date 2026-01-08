# Publishing Releases with GoReleaser

This project uses [GoReleaser](https://goreleaser.com/) to automate releases and Homebrew formula updates.

## Initial Setup

### 1. Install GoReleaser

```bash
brew install goreleaser
```

### 2. Create GitHub Personal Access Token

Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)

Create a token with these scopes:
- `repo` (full control of private repositories)

### 3. Authenticate

```bash
export GITHUB_TOKEN="your_token_here"
```

Or use `gh` CLI:
```bash
gh auth login
```

## Making a Release

### Step 1: Tag the Release

```bash
# Create and push tag
git tag v0.1.0
git push origin v0.1.0
```

### Step 2: Run GoReleaser

```bash
# Test release (doesn't publish)
goreleaser release --snapshot --clean

# Actual release (publishes to GitHub and updates Homebrew tap)
goreleaser release --clean
```

GoReleaser will:
- Build binaries for multiple platforms (linux/darwin/windows, amd64/arm64)
- Create a GitHub release with the binaries
- Generate and upload checksums
- Auto-update the Homebrew formula in your tap

### Step 3: Verify

Check the release on GitHub and verify the Homebrew formula was updated:
```bash
brew tap ghazimuharam/relocate
brew upgrade relocate
```

## What GoReleaser Does

1. **Builds cross-platform binaries**: Linux, macOS, Windows (amd64/arm64)
2. **Creates GitHub release**: Uploads all binaries and checksums
3. **Updates Homebrew tap**: Automatically updates the formula in `homebrew-relocate`
4. **Generates changelog**: Based on git commit messages

## Homebrew Tap

The Homebrew formula is automatically maintained at:
- Repository: `ghazimuharam/homebrew-relocate`
- Formula: `Formula/relocate.rb`

You **don't need to manually create or update this** - GoReleaser handles it!

### Users Install via:

```bash
brew tap ghazimuharam/relocate
brew install relocate
```

## Project Structure

```
.
├── .goreleaser.yaml    # GoReleaser configuration
├── cmd/
│   └── relocate/        # Main application entry point
│       └── main.go
├── internal/
│   └── config/          # Internal packages
│       └── config.go
├── go.mod
└── config.example.json  # Included in releases
```

## Configuration Notes

The `.goreleaser.yaml` file:
- Uses `ldflags` to inject version, commit, and date into the binary
- Builds for common platforms (linux/darwin/windows on amd64/arm64)
- Includes README and config.example.json in releases
- Auto-publishes to your Homebrew tap

## Version Info

To add version info to your binary, access these variables in `main.go`:

```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

These are automatically set by GoReleaser during release builds.

## Testing Before Release

Always test the release locally first:

```bash
# Build snapshot locally
goreleaser release --snapshot --clean

# Test the built binary
./dist/relocate_darwin_amd64/relocate --version
```
