# poe.ninja CLI

**Every poe.ninja price, offline, searchable, and scriptable — with arbitrage math no browser tab can do.**

poe-ninja-pp-cli syncs the full poe.ninja economy database to local SQLite and runs price queries, trend analysis, and arbitrage calculations in milliseconds. Find profitable currency flips, detect supply shocks before they spike, and calculate your liquid net worth — all from your terminal.

## Install

The recommended path installs both the `poe-ninja-pp-cli` binary and the `pp-poe-ninja` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install poe-ninja
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install poe-ninja --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install poe-ninja --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install poe-ninja --agent claude-code
npx -y @mvanhorn/printing-press-library install poe-ninja --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/poe-ninja/cmd/poe-ninja-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/poe-ninja-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-poe-ninja --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-poe-ninja --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-poe-ninja skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-poe-ninja. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/poe-ninja-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/poe-ninja/cmd/poe-ninja-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "poe-ninja": {
      "command": "poe-ninja-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify setup and API connectivity
poe-ninja-pp-cli doctor --dry-run

# Download PoE1 economy data for current league
poe-ninja-pp-cli sync --param league=Mirage --param type=Currency --full

# Check the current chaos value of Divine Orb
poe-ninja-pp-cli currency get "Divine Orb" --league Mirage

# Find the best round-trip currency trade with 500 chaos
poe-ninja-pp-cli flip --budget 500 --league Mirage

# Discover profitable multi-hop arbitrage cycles
poe-ninja-pp-cli arb --league Mirage --min-gain 5 --agent

```

## Unique Features

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

## Recipes


### Check currency price in current league

```bash
poe-ninja-pp-cli currency get "Divine Orb" --league Mirage --json
```

Returns chaos value, sparkline, and listing count for Divine Orb in Mirage league

### Find best currency flip with 500 chaos

```bash
poe-ninja-pp-cli flip --budget 500 --league Mirage --agent --select currency_from,currency_to,profit_chaos,bulk_minimum
```

Scans all currency pairs and returns the single highest-profit round-trip trade

### Detect supply shocks in the last 24 hours

```bash
poe-ninja-pp-cli shock --since 24h --league Mirage --min-drop 20 --agent --select name,listing_drop_pct,price_change_pct
```

Lists items where supply dropped more than 20% but price has not reacted yet

### Spot heating categories by price momentum

```bash
poe-ninja-pp-cli velocity --league Mirage --top 10 --agent --select category,velocity_cd_per_day,signal
```

Ranks item categories by price acceleration to reveal momentum before it peaks

### Decode div card profitability

```bash
poe-ninja-pp-cli divcard list --league Mirage --min-profit 10 --agent --select name,stack_cost_chaos,reward_value_chaos,profit_chaos
```

Shows cards where completing the set beats selling individual cards by 10 or more chaos

## Usage

Run `poe-ninja-pp-cli --help` for the full command reference and flag list.

## Commands

### currency

Currency exchange rates and chaos values (Path of Exile 1)

- **`poe-ninja-pp-cli currency`** - Get all currency exchange rates and chaos values for a league

### currency_poe2

Currency exchange rates for Path of Exile 2

- **`poe-ninja-pp-cli currency-poe2`** - Get currency exchange rates for a Path of Exile 2 league

### item

Item prices and market data (Path of Exile 1)

- **`poe-ninja-pp-cli item`** - Get item prices for a specific category and league


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
poe-ninja-pp-cli currency --league example-value --type Currency

# JSON for scripting and agents
poe-ninja-pp-cli currency --league example-value --type Currency --json

# Filter to specific fields
poe-ninja-pp-cli currency --league example-value --type Currency --json --select id,name,status

# Dry run — show the request without sending
poe-ninja-pp-cli currency --league example-value --type Currency --dry-run

# Agent mode — JSON + compact + no prompts in one flag
poe-ninja-pp-cli currency --league example-value --type Currency --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
poe-ninja-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Empty results from currency list** — Run sync --league LEAGUE first to populate the local database
- **Price data seems stale** — Run sync --league LEAGUE to refresh; poe.ninja updates every few minutes
- **league not found error** — Run leagues to see current valid league names; names are case-sensitive
- **arb or flip shows no results** — Ensure sync ran recently; rates change fast and stale data yields no arbitrage

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**poe-currency-flip-planner**](https://github.com/maximumstock/poe-currency-flip-planner) — Python (312 stars)
- [**poeapi**](https://github.com/willroberts/poeapi) — Go (127 stars)
- [**poe-api-manager**](https://github.com/ayberkgezer/poe-api-manager) — TypeScript (92 stars)
- [**poe_ninja_client**](https://github.com/infernumx/poe_ninja_client) — Python (48 stars)
- [**POEMCP**](https://github.com/shalayiding/POEMCP) — Python (34 stars)
- [**Path-of-Exile-Currency-Tracker**](https://github.com/Shad0wgale/Path-of-Exile-Currency-Tracker) — Python (15 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
