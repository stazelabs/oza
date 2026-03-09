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

Quit and reopen Claude Desktop. A hammer icon in the chat input area confirms tools are loaded. Click it to verify you see `list_archives`, `search_text`, `read_entry`, and the other tools.

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

Seven tools are available. Start with `list_archives` to discover archives, then use `search_text` or `browse_titles` to find entries, `get_entry_info` to triage without reading content, and `read_entry` to fetch full content.

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

### `get_entry_info`

Return metadata for an entry without fetching its content. Use this to triage results before deciding whether to call `read_entry`.

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `archive` | string | yes | Archive slug |
| `entry_id` | int | no | Entry ID (provide entry_id or path) |
| `path` | string | no | Entry path (provide entry_id or path) |

**Output:** JSON object with `entry_id`, `path`, `title`, `mime_type`, `size_bytes`, `is_redirect`, `is_front_article`. With `ozaserve --mcp`, includes a `url` field.

### `browse_titles`

Browse archive entries in alphabetical title order with offset/limit pagination.

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `archive` | string | yes | Archive slug |
| `offset` | int | no | Starting position in the sorted title list (default 0) |
| `limit` | int | no | Max entries to return (default 50, max 200) |

**Output:** JSON object with `total_count`, `offset`, `count`, and an `entries` array of `{entry_id, path, title, mime_type, is_front_article}`. With `ozaserve --mcp`, each entry includes a `url` field.

### `get_random`

Return a random front-article entry. Useful for exploration and serendipity.

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `archive` | string | no | Archive slug (omit to pick from any loaded archive) |

**Output:** JSON object with `archive`, `entry_id`, `path`, `title`, `mime_type`. With `ozaserve --mcp`, includes a `url` field. Follow up with `read_entry` to fetch the content.

### `get_archive_stats`

Return detailed statistics for an archive: entry counts and uncompressed sizes by MIME type, chunk count, and section inventory. More detailed than `list_archives`.

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `archive` | string | yes | Archive slug |

**Output:** JSON object with `entry_count`, `redirect_count`, `chunk_count`, `has_search`, a `mime_stats` array (sorted by entry count), and a `sections` array with compressed/uncompressed sizes and compression method for each section.

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

## Getting Claude to Use OZA Resources

By default Claude may answer from training data instead of the archive. These prompting patterns reliably direct it to the OZA tools first.

### Use the archive explicitly

The simplest approach: name the archive in your request.

> Use oza to create a short lesson plan for basic programming. Provide links to browse useful articles.

> Using the encyclopedia, explain how nuclear fusion works.

> According to the Wikipedia archive, who was Marie Curie?

> Look up "plate tectonics" in the archive and summarize it for me.

### Instruct Claude to check the archive before answering

Add a standing instruction at the start of the conversation:

> Before answering any factual questions, always search the OZA archive first and cite the article you used.

Or frame it as a constraint:

> Answer only from the encyclopedia. Do not use your training data.

### Embed the instruction in a system prompt (API / Claude Code)

When calling the API directly, set a system prompt that establishes the archive as the authority:

```
You have access to a local OZA encyclopedia via the list_archives, search_text,
and read_entry MCP tools. Always consult these tools before answering factual
questions. Cite the article title and path in every response.
```

### Ask Claude to read a specific entry by path

Skip search entirely and go straight to a known article:

> Read the entry at path "Photosynthesis" from the wikipedia archive.

> Use read_entry to get the "Quantum_mechanics" article and explain it simply.

### Ask for citations to force source retrieval

Requiring citations forces Claude to actually fetch content rather than paraphrase from memory:

> Explain the water cycle. Quote at least two sentences directly from the encyclopedia article and include the source link.

### CLAUDE.md for Claude Code sessions

If you use Claude Code, add a standing instruction to `CLAUDE.md` in your project:

```markdown
## Reference Material

An OZA encyclopedia is available via MCP tools (list_archives, search_text,
read_entry). Consult it for factual background on the project domain before
answering questions or making implementation decisions.
```

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
