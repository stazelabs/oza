# OZA Extension Section Type Registry

This document is the authoritative registry for allocated OZA section type values in the
spec-sanctioned extension range (`0x0100–0x01FF`). It also records the conventions for
vendor/community and private extension ranges.

See §1 of `FORMAT.md` ("Extension sections across major versions") for the full
namespace breakdown and writer/reader obligations.

---

## Range summary

| Range           | Purpose                    | Vendor prefix required? |
|-----------------|----------------------------|-------------------------|
| `0x0100–0x01FF` | Spec-sanctioned extensions | No                      |
| `0x0200–0xFEFF` | Vendor/community           | Yes — 4-byte ASCII      |
| `0xFF00–0xFFFF` | Private/experimental       | N/A (not distributable) |

---

## Spec-sanctioned allocations (`0x0100–0x01FF`)

*None allocated yet.*

| Type | Name | Since | Description |
|------|------|-------|-------------|
| —    | —    | —     | —           |

---

## Requesting a new spec-sanctioned allocation

To request a value in the `0x0100–0x01FF` range, open a pull request against this
repository that:

1. Adds a row to the table above with the proposed type value, a short ALL_CAPS name,
   the OZA spec version it targets, and a one-line description.
2. Updates `FORMAT.md` section §3.3 (section type table) with the same row.
3. Includes a short design note (inline or in a linked document) covering:
   - What the section contains and why it cannot be expressed with existing sections.
   - Reader behaviour when the section is absent (i.e. is it `SECTION_CRITICAL` or
     skippable?).
   - Whether the section is intended for a specific major version or is version-agnostic.

Allocations are granted by the OZA maintainers via PR review. There is no formal
standards body — consensus among active implementors is sufficient.

---

## Vendor/community extensions (`0x0200–0xFEFF`)

Organisations may use any value in this range without prior registration. To prevent
collisions, the first 4 bytes of the section payload MUST be a 4-byte ASCII vendor
prefix that uniquely identifies the organisation or project (e.g. `KWIX` for Kiwix,
`WIKI` for Wikimedia, `STAZ` for Stazelabs).

Known vendor prefixes (informational, not exhaustive):

| Prefix | Organisation / project |
|--------|------------------------|
| —      | —                      |

To list your prefix here, open a PR adding a row to the table above. This is voluntary
and informational only — it does not reserve or allocate any type value.

---

## Private/experimental range (`0xFF00–0xFFFF`)

Values in this range MUST NOT appear in archives intended for distribution. They are
reserved for local testing, draft implementations, and tooling prototypes. No
registration is required or possible.