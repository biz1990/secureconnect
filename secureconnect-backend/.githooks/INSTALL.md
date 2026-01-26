# Pre-commit Hooks Installation

This directory contains Git pre-commit hooks for the SecureConnect backend project.

## Installation

### Linux/macOS (Bash)

1. Make the pre-commit script executable:
```bash
chmod +x .githooks/pre-commit
```

2. Install the hook:
```bash
git config core.hooksPath .githooks
```

Or manually copy:
```bash
cp .githooks/pre-commit .git/hooks/
chmod +x .git/hooks/pre-commit
```

### Windows (Batch)

1. Install the hook:
```cmd
git config core.hooksPath .githooks
```

Or manually copy:
```cmd
copy .githooks\pre-commit.bat .git\hooks\pre-commit
```

## What the Hook Does

The pre-commit hook runs the following checks before each commit:

1. **go fmt** - Ensures code is properly formatted
2. **go vet** - Runs Go's static analysis tool
3. **golangci-lint** - Runs comprehensive linting (if installed)
4. **go test** - Runs unit tests (short tests only)

## Skipping the Hook

If you need to skip the pre-commit hooks for a specific commit:

```bash
git commit --no-verify -m "Your commit message"
```

## Installing golangci-lint (Optional but Recommended)

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Troubleshooting

If the hook doesn't run after installation:

1. Check if hooks are enabled:
```bash
git config core.hooksPath
```

2. Verify the hook is executable (Linux/macOS):
```bash
ls -la .githooks/pre-commit
```

3. Test the hook manually:
```bash
./.githooks/pre-commit
```
