package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type divcardEntry struct {
	Name         string  `json:"name"`
	StackSize    int     `json:"stack_size"`
	CardPrice    float64 `json:"card_price_chaos"`
	SetCost      float64 `json:"set_cost_chaos"`
	ValueEach    float64 `json:"card_value_each_chaos"`
	TrendChange  float64 `json:"trend_7d_pct"`
	ListingCount int     `json:"listing_count"`
	FlavourText  string  `json:"flavour_text,omitempty"`
}

type divcardView struct {
	League string         `json:"league"`
	Cards  []divcardEntry `json:"cards"`
	Note   string         `json:"note"`
}

func newNovelDivcardCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagMinProfit float64
	var flagLimit int
	var flagSort string

	cmd := &cobra.Command{
		Use:   "divcard",
		Short: "Show all divination cards sorted by value, set cost, and trend — find which card sets are worth completing.",
		Long: `Fetches all divination cards from poe.ninja and computes:
- Card price (chaos each)
- Set cost (card price × stack size = total chaos to buy a full set)
- Value density (chaos per card slot)
- 7-day trend

Note: poe.ninja does not expose the reward item price in the API, so set completion
profit requires comparing set_cost against the reward item's price manually.
Use --sort set-cost to find cheapest sets to farm, or --sort value to find
the most valuable individual cards to sell.`,
		Example: strings.Trim(`
  poe-ninja-pp-cli divcard list --league Mirage
  poe-ninja-pp-cli divcard list --league Mirage --sort set-cost --min-value 5
  poe-ninja-pp-cli divcard list --league Mirage --json --agent --select name,set_cost_chaos,trend_7d_pct`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
	}

	listCmd := &cobra.Command{
		Use:         "list",
		Short:       "List all divination cards ranked by value or set cost",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && flagLeague == "Mirage" {
				// Not a bare invocation — proceed with defaults
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list divination cards for league:", flagLeague)
				return nil
			}
			if flagLeague == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required"))
			}

			ir, err := poeFetchItem(flagLeague, "DivinationCard")
			if err != nil {
				return apiErr(fmt.Errorf("fetching divination card data: %w", err))
			}

			var cards []divcardEntry
			for _, l := range ir.Lines {
				if l.ChaosValue < flagMinProfit {
					continue
				}
				stackSize := l.StackSize
				if stackSize <= 0 {
					stackSize = 1
				}
				cards = append(cards, divcardEntry{
					Name:         l.Name,
					StackSize:    stackSize,
					CardPrice:    l.ChaosValue,
					SetCost:      l.ChaosValue * float64(stackSize),
					ValueEach:    l.ChaosValue,
					TrendChange:  l.SparkLine.TotalChange,
					ListingCount: l.ListingCount,
					FlavourText:  l.FlavourText,
				})
			}

			switch flagSort {
			case "set-cost":
				sort.Slice(cards, func(i, j int) bool {
					return cards[i].SetCost < cards[j].SetCost
				})
			case "trend":
				sort.Slice(cards, func(i, j int) bool {
					return cards[i].TrendChange > cards[j].TrendChange
				})
			default: // "value"
				sort.Slice(cards, func(i, j int) bool {
					return cards[i].CardPrice > cards[j].CardPrice
				})
			}

			if flagLimit > 0 && len(cards) > flagLimit {
				cards = cards[:flagLimit]
			}

			note := "set_cost = card_price x stack_size. Compare set_cost against the reward item price to find profitable sets to complete."
			view := divcardView{League: flagLeague, Cards: cards, Note: note}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nDivination Cards — %s league (sorted by %s)\n\n", flagLeague, flagSort)
			fmt.Fprintf(w, "  %-32s %6s %10s %12s %10s %10s\n", "Card", "Stack", "Per Card", "Set Cost", "7d Change", "Listings")
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 84))
			for _, c := range cards {
				sign := "+"
				if c.TrendChange < 0 {
					sign = ""
				}
				fmt.Fprintf(w, "  %-32s %6d %9.2fc %11.2fc %9s%.1f%% %10d\n",
					c.Name, c.StackSize, c.CardPrice, c.SetCost, sign, c.TrendChange, c.ListingCount)
			}
			fmt.Fprintf(w, "\n  Note: %s\n\n", note)
			return nil
		},
	}
	listCmd.Flags().StringVar(&flagLeague, "league", "Mirage", "League name (e.g. Mirage, Standard)")
	listCmd.Flags().Float64Var(&flagMinProfit, "min-profit", 1.0, "Minimum card chaos value to include")
	listCmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum results to show")
	listCmd.Flags().StringVar(&flagSort, "sort", "value", "Sort order: value, set-cost, or trend")

	cmd.AddCommand(listCmd)
	// Default: run list command when called bare
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return c.Help()
		}
		return nil
	}
	return cmd
}
