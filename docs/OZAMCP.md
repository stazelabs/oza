# MCP Server for OZA Archives

OZA archives can be exposed as tools and resources for LLMs via the [Model Context Protocol](https://modelcontextprotocol.io). There are two ways to run the MCP server:

- **`ozaserve --mcp`** (recommended) — Runs both HTTP and MCP. Tool results include clickable URLs that link to the rendered content in your browser.
- **`ozamcp`** — Lightweight standalone MCP server (no HTTP, no URLs in results).

---

## ozaserve --mcp (Recommended)

Runs the HTTP content server and MCP server simultaneously. The MCP tools embed browsable URLs (`http://localhost:8080/{slug}/{path}`) in their results, so Claude can cite sources with clickable links.

```bash
ozaserve [file.oza ...] [--dir <dir>] --mcp [flags]
```

### Claude Desktop Setup

#### 1. Build

```bash
cd /path/to/oza
make build
```

#### 2. Configure

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

**Single archive:**

```json
{
  "mcpServers": {
    "oza": {
      "command": "/absolute/path/to/bin/ozaserve",
      "args": [
        "/absolute/path/to/wikipedia_en_simple_all_maxi_2026-02.oza",
        "--mcp"
      ]
    }
  }
}
```

**Multiple archives or directory:**

```json
{
  "mcpServers": {
    "oza": {
      "command": "/absolute/path/to/bin/ozaserve",
      "args": [
        "--dir", "/absolute/path/to/oza-archives/",
        "--recursive",
        "--mcp"
      ]
    }
  }
}
```

**Custom port:**

```json
{
  "mcpServers": {
    "oza": {
      "command": "/absolute/path/to/bin/ozaserve",
      "args": [
        "/absolute/path/to/archive.oza",
        "--mcp",
        "--addr", ":9090"
      ]
    }
  }
}
```

All paths must be absolute.

#### 3. Restart Claude Desktop

Quit and reopen Claude Desktop. A hammer icon in the chat input area confirms tools are loaded. Click it to verify you see `list_archives`, `search_text`, and `read_entry`.

While Claude Desktop is connected, you can also browse archives at `http://localhost:8080` in your browser.

---

## ozamcp (Standalone)

Lightweight MCP-only server. No HTTP server, no URLs in tool results.

```bash
ozamcp [file.oza ...] [--dir <dir>] [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dir`, `-d` | | Directory of OZA files (repeatable) |
| `--recursive`, `-r` | `false` | Scan `--dir` directories recursively |
| `--transport`, `-t` | `stdio` | Transport: `stdio` or `sse` |
| `--cache`, `-c` | `64` | Chunk cache size per archive |

### Claude Desktop Config

```json
{
  "mcpServers": {
    "oza": {
      "command": "/absolute/path/to/bin/ozamcp",
      "args": [
        "/absolute/path/to/wikipedia_en_simple_all_maxi_2026-02.oza"
      ]
    }
  }
}
```

---

## Tools

### `list_archives`

List all loaded OZA archives with metadata.

**Input:** none

**Output:** JSON array of archives with slug, title, description, UUID, entry count, and capabilities. With `ozaserve --mcp`, each archive includes a browsable `url` field.

### `search_text`

Trigram text search across archives.

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Search query (min 3 chars) |
| `archive` | string | no | Archive slug (omit to search all) |
| `limit` | int | no | Max results (default 20) |

**Output:** JSON array of hits with archive, entry_id, path, title, and match indicators. With `ozaserve --mcp`, each hit includes a browsable `url` field.

### `read_entry`

Read entry content as clean markdown (or raw HTML).

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `archive` | string | yes | Archive slug |
| `entry_id` | int | no | Entry ID (provide entry_id or path) |
| `path` | string | no | Entry path (provide entry_id or path) |
| `format` | string | no | `markdown` (default) or `html` |

**Output:** Entry content with a title header and source link. HTML is converted to markdown by default. With `ozaserve --mcp`, the header includes a clickable source URL.

---

## Resources

| URI | Description |
|-----|-------------|
| `oza://{slug}/metadata` | Archive metadata as JSON |
| `oza://{slug}/entry/{id}` | Entry content as markdown |

---

## Sample Workflows

### Discover and browse

> **You:** What archives do you have access to?

Claude calls `list_archives`:

> I have access to 1 OZA archive:
> - **Wikipedia in simple English** — 613,061 entries, with text search available.
>   Browse: http://localhost:8080/wikipedia_en_simple_all_maxi/

> **You:** Search for articles about black holes.

Claude calls `search_text` with `{"query": "black holes", "limit": 10}`:

> I found several results:
> 1. [**Black hole**](http://localhost:8080/wikipedia_en_simple_all_maxi/A/Black_hole) (entry 12345)
> 2. [**Supermassive black hole**](http://localhost:8080/wikipedia_en_simple_all_maxi/A/Supermassive_black_hole) (entry 23456)
> 3. [**Hawking radiation**](http://localhost:8080/wikipedia_en_simple_all_maxi/A/Hawking_radiation) (entry 34567)

> **You:** Read the black hole article.

Claude calls `read_entry` with `{"archive": "wikipedia_en_simple_all_maxi", "entry_id": 12345}` and returns the full article as clean markdown with a source link.

### Research question

> **You:** Using the encyclopedia, explain how photosynthesis works. Cite your sources.

Claude will search for "photosynthesis", read the top results, and synthesize an answer citing specific articles with links.

### Compare topics

> **You:** Compare and contrast mitosis and meiosis using the encyclopedia.

Claude will search and read both articles, then write a structured comparison with source links.

---

## Permissions

When Claude wants to use a tool, Claude Desktop shows a permission prompt:

- **Allow once** — approve this single call
- **Allow for this chat** — auto-approve this tool for the rest of the conversation
- **Deny** — block the call

---

## Troubleshooting

**Tools don't appear in Claude Desktop:**

1. Verify the archive loads:
   ```bash
   ./bin/ozaserve /path/to/archive.oza --mcp
   # Should print: loaded: /slug/ — Title (N entries)
   # Then: MCP server running on stdio
   ```
2. Check config JSON is valid (no trailing commas, absolute paths).
3. Check Claude Desktop logs: `~/Library/Logs/Claude/`
4. Restart Claude Desktop after any config change.

**"index section bad magic" error:**

The archive was built with an older OZA format. Rebuild it with the current `zim2oza`.

**HTTP server not accessible:**

When using `ozaserve --mcp`, the HTTP server starts on `:8080` by default. Use `--addr` to change the port if it's already in use.
