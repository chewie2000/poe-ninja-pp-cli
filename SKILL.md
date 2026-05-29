---
name: pp-poe-ninja
description: "Every poe.ninja price, offline, searchable, and scriptable — with arbitrage math no browser tab can do. Trigger phrases: `check the price of divine orbs`, `what can I flip with 500 chaos`, `find arbitrage in poe`, `what items are spiking in price`, `how much is my poe stash worth`, `use poe-ninja`, `run poe-ninja`."
author: "chewie2000"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - poe-ninja-pp-cli
    install:
      - kind: go
        bins: [poe-ninja-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/poe-ninja/cmd/poe-ninja-pp-cli
---

# poe.ninja — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `poe-ninja-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install poe-ninja --cli-only
   ```
2. Verify: `poe-ninja-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/poe-ninja/cmd/poe-ninja-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

poe-ninja-pp-cli syncs the full poe.ninja economy database to local SQLite and runs price queries, trend analysis, and arbitrage calculations in milliseconds. Find profitable currency flips, detect supply shocks before they spike, and calculate your liquid net worth — all from your terminal.

## When to Use This CLI

Use poe-ninja-pp-cli when you need fast, scriptable access to Path of Exile economy data — checking currency prices, finding flip opportunities, detecting market trends, or calculating net worth. Ideal for traders, build-planners comparing item costs across leagues, and automation scripts that need structured price data.

## Anti-triggers

Do not use this CLI for:
- Reading or writing a Path of Exile account stash (use the official GGG API with OAuth)
- Executing actual in-game trades (use the official trade site)
- Fetching item mods or passive tree data (use poewiki or poedb)
- Path of Exile 2 skill gem or passive tree data (poe.ninja only provides economy/price data)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Profit intelligence
- **`arb`** — Find profitable multi-hop currency trading cycles using graph math over all exchange rates simultaneously.

  _When a user asks which currency trades are profitable right now, arb finds cycles a human would never spot manually._

  ```bash
  poe-ninja-pp-cli arb --budget 500 --league Mirage --min-gain 5 --agent
  ```
- **`flip`** — Given a chaos budget, find the single most profitable currency-to-currency round-trip trade accounting for bulk minimums.

  _When a user has a fixed amount of chaos and wants to grow it in one trade, flip finds the best option instantly._

  ```bash
  poe-ninja-pp-cli flip --budget 500 --league Mirage --agent
  ```
- **`divcard`** — Show all divination cards sorted by profit from completing a set vs. selling individual cards, using cross-table price joins.

  _When a user finds a divination card stack and wants to know whether to complete or sell it, divcard gives the exact profit calculation._

  ```bash
  poe-ninja-pp-cli divcard list --league Mirage --min-profit 10 --agent
  ```

### Market intelligence
- **`shock`** — Find items where supply is drying up (listing count dropped) before price has reacted — the leading edge of a price spike.

  _When an agent needs to find items about to spike in price, shock surfaces divergences between supply and price before the market corrects._

  ```bash
  poe-ninja-pp-cli shock --since 24h --league Mirage --min-drop 20 --agent
  ```
- **`velocity`** — Rank item categories by price acceleration computed from 7-day sparkline data across the entire database.

  _When an agent needs to know which item categories are heating up right now, velocity surfaces momentum signals invisible to per-item browsing._

  ```bash
  poe-ninja-pp-cli velocity --league Mirage --top 20 --agent
  ```
- **`trend`** — Render 7-day price history for any currency or item as an ASCII chart directly in the terminal.

  _When a user wants to see if a price is trending up or down, trend gives an instant visual without opening a browser._

  ```bash
  poe-ninja-pp-cli trend "Divine Orb" --league Mirage
  ```
- **`compare`** — Show the price of the same item side-by-side in two leagues to identify value arbitrage opportunities.

  _When a user wants to know if an item is cheaper in Standard, compare surfaces the delta instantly._

  ```bash
  poe-ninja-pp-cli compare "Divine Orb" --league1 Mirage --league2 Standard --agent
  ```

### Portfolio management
- **`networth`** — Calculate total chaos value of a user-supplied inventory, flagging illiquid holdings (low listing count) vs. paper wealth.

  _When a user wants to know their actual liquid worth (not paper wealth), networth separates sellable value from stuck inventory._

  ```bash
  poe-ninja-pp-cli networth --items "Divine Orb:50,Mirror of Kalandra:1" --league Mirage --agent
  ```
- **`portfolio`** — Track a user-defined list of currencies and items by total chaos value, updated each sync.

  _When a user wants to track their wealth over time, portfolio converts a mixed inventory to comparable chaos value._

  ```bash
  poe-ninja-pp-cli portfolio --items "Divine Orb:50,Mirror of Kalandra:1" --league Mirage --agent
  ```

## Command Reference

**currency** — Currency exchange rates and chaos values (Path of Exile 1)

- `poe-ninja-pp-cli currency` — Get all currency exchange rates and chaos values for a league

**currency_poe2** — Currency exchange rates for Path of Exile 2

- `poe-ninja-pp-cli currency-poe2` — Get currency exchange rates for a Path of Exile 2 league

**item** — Item prices and market data (Path of Exile 1)

- `poe-ninja-pp-cli item` — Get item prices for a specific category and league


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
poe-ninja-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Check 7-day price trend for Divine Orb

```bash
poe-ninja-pp-cli trend "Divine Orb" --league Mirage --json
```

Returns chaos value, sparkline chart, and 7-day change % for Divine Orb in Mirage league

### Find best currency flip with 500 chaos

```bash
poe-ninja-pp-cli flip --budget 500 --league Mirage --agent --select currency_from,currency_to,profit_chaos,profit_pct
```

Scans all currency pairs and returns the single highest-profit round-trip trade

### Detect supply shocks in the last 24 hours

```bash
poe-ninja-pp-cli shock --league Mirage --min-drop 20 --agent --select name,total_change_pct,listing_count,category
```

Lists items where price dropped more than 20% — a leading supply shock indicator

### Spot heating categories by price momentum

```bash
poe-ninja-pp-cli velocity --league Mirage --top 10 --agent --select category,velocity_cd_per_day,signal
```

Ranks item categories by price acceleration to reveal momentum before it peaks

### Decode div card profitability

```bash
poe-ninja-pp-cli divcard list --league Mirage --min-profit 5 --agent --select name,card_price_chaos,set_cost_chaos,trend_7d_pct
```

Shows all divination cards ranked by price with set cost and 7-day price trend

## Auth Setup

No authentication required.

Run `poe-ninja-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  poe-ninja-pp-cli currency --league example-value --type Currency --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
poe-ninja-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
poe-ninja-pp-cli feedback --stdin < notes.txt
poe-ninja-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/poe-ninja-pp-cli/feedback.jsonl`. They are never POSTed unless `POE_NINJA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POE_NINJA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
poe-ninja-pp-cli profile save briefing --json
poe-ninja-pp-cli --profile briefing currency --league example-value --type Currency
poe-ninja-pp-cli profile list --json
poe-ninja-pp-cli profile show briefing
poe-ninja-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `poe-ninja-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/poe-ninja/cmd/poe-ninja-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add poe-ninja-pp-mcp -- poe-ninja-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which poe-ninja-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   poe-ninja-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `poe-ninja-pp-cli <command> --help`.
