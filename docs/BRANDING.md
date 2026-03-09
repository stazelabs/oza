# 王座 OZA -- Branding Guide

## Identity

| Element | Value |
|---------|-------|
| **Name** | OZA |
| **Full name** | Open Zipped Archive |
| **Kanji** | 王座 |
| **Characters** | 王 (king) + 座 (seat/position) |
| **Meaning** | Throne, royal seat |
| **Reading** | Japanese: oza (おうざ) |
| **File extension** | `.oza` |
| **MIME type** | `application/x-oza` |

## The Name

王座 (oza) means "throne" in Japanese -- the seat of authority. The name reflects the project's ambition: OZA takes the throne as the successor to ZIM, the format that has served the offline content community for nearly two decades.

The phonetic alignment is direct. The Japanese reading of 王座 is "oza," which maps exactly to the project acronym OZA (Open Zipped Archive). This is not a backronym -- the meaning and the acronym were chosen together.

## Metaphor

The throne metaphor works on multiple levels:

- **Succession.** A throne passes from one ruler to the next. OZA succeeds ZIM not through conflict but through evolution -- taking the hard-won lessons of two decades and building something better.
- **Authority.** The throne represents legitimacy and standard-setting. OZA aims to be the definitive format for offline content distribution.
- **Permanence.** Thrones endure. OZA is designed with extensibility (the section table) so it can evolve without the brittle freezing that afflicted ZIM's header.
- **Seat of power.** 座 specifically means "seat" or "place" -- a fitting word for a format that gives content a permanent home offline.

## Visual Elements

### CLI Banner

```
王座 OZA -- Open Zipped Archive
```

Used at the top of `--help` output and verbose mode headers.

### Tool Version Output

```
王座 ozainfo v0.1.0
王座 ozacat v0.1.0
王座 ozaserve v0.1.0
王座 ozasearch v0.1.0
王座 ozaverify v0.1.0
王座 zim2oza v0.1.0
```

### ASCII Box (README, docs)

```
  ╔═══════════════════════════════╗
  ║          王  座               ║
  ║           OZA                 ║
  ║   Open Zipped Archive v1.0   ║
  ╚═══════════════════════════════╝
```

## Usage in Code

- **Error prefix:** `oza:` (e.g., `oza: invalid magic number`)
- **Package doc:** `// Package oza provides a pure Go implementation for reading OZA archives.`
- **Go module:** `github.com/stazelabs/oza`
- **Binary names:** `ozainfo`, `ozacat`, `ozaserve`, `ozasearch`, `ozaverify`, `zim2oza`

## Tagline

Primary: **王座 -- The Throne**

Alternatives for different contexts:
- "A modern crown for offline content"
- "The throne of offline archives"
- "Taking the throne from ZIM"

## Color (if applicable)

No mandatory color scheme. When used in contexts that support color:
- Primary: Deep gold (#C9A84C) -- throne/royalty association
- Secondary: Charcoal (#2D2D2D) -- technical authority
- Accent: White (#FFFFFF) -- clarity

## Do / Don't

**Do:**
- Use 王座 alongside OZA in headers and titles
- Include 王座 in CLI `--version` output
- Use the kanji as a distinctive visual mark

**Don't:**
- Use 王座 as the sole identifier (always pair with "OZA")
- Translate 王座 to English in code-facing contexts (keep the kanji)
- Use the kanji in file names or paths (stick to ASCII: `oza`, not `王座`)
