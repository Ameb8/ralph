# Ralph

Ralph is an automated issue-driven development CLI tool.

## Build

Compile the project using standard Go build commands:

```bash
go build -o bin/ralph ./cmd/ralph
```

## Run

Run the unified PR pipeline by passing one or more issue numbers:

```bash
./bin/ralph run <issue_1> [<issue_2> ...]
```

**Example:**
```bash
./bin/ralph run 42 45
```
