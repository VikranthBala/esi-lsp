# akamai-esi-lsp

A Language Server Protocol (LSP) implementation for **Akamai ESI (Edge Side Includes)**, written in Go.

ESI has no RFC — this server targets the **Akamai ESI dialect specifically**.

---

## Features

| Feature                          | Status         |
|----------------------------------|----------------|
| Diagnostics / validation         | 🚧 In progress |
| Autocomplete / IntelliSense      | 🚧 In progress |
| Hover documentation              | 🚧 In progress |
| Go-to-definition                 | 🚧 In progress |

---

## Supported ESI Tags

| Tag                  | Description                                        |
|----------------------|----------------------------------------------------|
| `<esi:include>`      | Fetch and include another resource                 |
| `<esi:remove>`       | Remove content at ESI processing time              |
| `<esi:comment>`      | ESI-only comment, stripped at processing           |
| `<esi:vars>`         | Evaluate ESI variables in a block                  |
| `<esi:assign>`       | Assign a value to a variable                       |
| `<esi:eval>`         | Evaluate another ESI page inline                   |
| `<esi:choose>`       | Conditional container (like switch)                |
| `<esi:when>`         | Branch within `esi:choose`                         |
| `<esi:otherwise>`    | Default branch within `esi:choose`                 |
| `<esi:try>`          | Error-handling container                           |
| `<esi:attempt>`      | Primary branch within `esi:try`                    |
| `<esi:except>`       | Fallback branch within `esi:try`                   |
| `<esi:inline>`       | Define an inline ESI fragment                      |
| `<esi:function>`     | Reusable ESI function (Akamai extension)           |
| `<esi:text>`         | Raw text block, no ESI processing inside           |

## Supported ESI Variables

| Variable                      | Description                    |
|-------------------------------|--------------------------------|
| `$(HTTP_COOKIE{name})`        | Cookie value by name           |
| `$(QUERY_STRING{name})`       | Query string parameter         |
| `$(HTTP_HOST)`                | Request Host header            |
| `$(REQUEST_PATH)`             | Request path                   |
| `$(REQUEST_METHOD)`           | HTTP method                    |
| `$(HTTP_ACCEPT_LANGUAGE)`     | Accept-Language header         |
| `$(GEO{country_code})`        | Akamai GeoIP country           |
| `$(GEO{region_code})`         | Akamai GeoIP region            |
| `$(USER_AGENT)`               | User-Agent string              |

---

## Prerequisites

- Go 1.22+
- Node.js 18+ and npm (for the VS Code extension)
- VS Code

---

## Building

```bash
# Clone
git clone https://github.com/yourname/akamai-esi-lsp
cd akamai-esi-lsp

# Download dependencies
go mod tidy

# Build the language server binary
go build -o esi-lsp ./cmd/esi-lsp
```

## Running tests

```bash
go test ./...
```

---

## How it works

ESI markup is embedded inside HTML that is often not well-formed XML.
A strict XML parser would reject the surrounding document, so we use a two-phase approach:

1. **Scan** the raw source for `<esi:` patterns to find candidate positions.
2. **Tokenize** the extracted ESI fragments using Go's `encoding/xml`.
3. **Track byte offsets** throughout to produce accurate `line:character` ranges for LSP responses.

The Go binary speaks **JSON-RPC over stdio** — the VS Code extension simply
launches it as a child process and connects as an LSP client.

---

## License

MIT


NOTE:

ONLY short form of tags are supported
