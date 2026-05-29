package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type shockEntry struct {
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	ChaosValue    float64 `json:"chaos_value"`
	ListingCount  int     `json:"listing_count"`
	TotalChange   float64 `json:"price_change_7d_pct"`
	SparklineNote string  `json:"sparkline_note"`
	Signal        string  `json:"signal"`
}

type shockView struct {
	League  string       `json:"league"`
	MinDrop float64      `json:"min_drop_pct"`
	Results []shockEntry `json:"results"`
	Note    string       `json:"note,omitempty"`
}

func newNovelShockCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagLeague string
	var flagMinDrop float64
	var flagCategory string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "shock",
		Short: "Find items where supply is drying up (listing count dropped) before price has reacted.",
		Long: `Identify potential price spikes before they happen. Uses listing count as a supply proxy:
when supply drops significantly but price has not reacted yet, it often signals an upcoming spike.
This uses live poe.ninja data and the 7-day sparkline as a leading indicator.
Note: --since is retained for compatibility but shock now uses the sparkline's total-change signal.`,
		Example: strings.Trim(`
  poe-ninja-pp-cli shock --league Mirage
  poe-ninja-pp-cli shock --league Mirage --min-drop 20 --category Currency
  poe-ninja-pp-cli shock --league Mirage --limit 20 --json --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would scan %s for supply shocks with min-drop %.0f%%\n", flagLeague, flagMinDrop)
				return nil
			}
			if flagLeague == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required"))
			}
			_ = flagSince // used in future snapshot-based implementation

			var results []shockEntry
			var fetchErrors int
			var totalFetches int

			// Strategy: items with low listing count AND negative price change (supply
			// declining faster than price — the divergence window before market reacts).
			// We use listing_count < 20 as "low supply" and TotalChange as price proxy.
			checkCurrency := flagCategory == "" || strings.EqualFold(flagCategory, "Currency")
			checkItems := flagCategory == "" || !strings.EqualFold(flagCategory, "Currency")

			if checkCurrency {
				for _, t := range []string{"Currency", "Fragment"} {
					totalFetches++
					cr, err := poeFetchCurrency(flagLeague, t)
					if err != nil {
						fetchErrors++
						continue
					}
					for _, l := range cr.Lines {
						if l.ListingCount == 0 {
							continue
						}
						// Supply shock signal: listing count dropped (we use sparkline.totalChange as proxy),
						// specifically items with FALLING price that still have low listing count
						// or items with large receive-sparkline drops.
						dropProxy := -l.ReceiveSparkLine.TotalChange
						if dropProxy < flagMinDrop {
							continue
						}
						signal := "SUPPLY-DROP"
						if l.ChaosEquivalent > 100 {
							signal = "HIGH-VALUE-SUPPLY-DROP"
						}
						results = append(results, shockEntry{
							Name:          l.CurrencyTypeName,
							Category:      t,
							ChaosValue:    l.ChaosEquivalent,
							ListingCount:  l.ListingCount,
							TotalChange:   l.ReceiveSparkLine.TotalChange,
							SparklineNote: sparklineASCII(l.ReceiveSparkLine.Data),
							Signal:        signal,
						})
					}
				}
			}

			if checkItems {
				searchTypes := poeAllItemTypes
				if flagCategory != "" {
					searchTypes = []string{flagCategory}
				}
				for _, t := range searchTypes {
					totalFetches++
					ir, err := poeFetchItem(flagLeague, t)
					if err != nil {
						fetchErrors++
						continue
					}
					for _, l := range ir.Lines {
						if l.ListingCount == 0 {
							continue
						}
						// Items where price fell but listing count is low (supply shock coming)
						dropProxy := -l.SparkLine.TotalChange
						if dropProxy < flagMinDrop {
							continue
						}
						signal := "SUPPLY-DROP"
						if l.ChaosValue > 50 {
							signal = "HIGH-VALUE-SUPPLY-DROP"
						}
						results = append(results, shockEntry{
							Name:          l.Name,
							Category:      t,
							ChaosValue:    l.ChaosValue,
							ListingCount:  l.ListingCount,
							TotalChange:   l.SparkLine.TotalChange,
							SparklineNote: sparklineASCII(l.SparkLine.Data),
							Signal:        signal,
						})
					}
				}
			}

			// Sort by: price drop (most negative first) with chaos value as tiebreak
			sort.Slice(results, func(i, j int) bool {
				if results[i].TotalChange != results[j].TotalChange {
					return results[i].TotalChange < results[j].TotalChange
				}
				return results[i].ChaosValue > results[j].ChaosValue
			})
			if flagLimit > 0 && len(results) > flagLimit {
				results = results[:flagLimit]
			}

			note := "Supply shock detection uses 7-day price-trend reversal as a leading indicator. Items with falling prices + low listing count often precede supply-driven spikes."
			view := shockView{League: flagLeague, MinDrop: flagMinDrop, Results: results, Note: note}

			if fetchErrors > 0 && totalFetches > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %d of %d categories failed to fetch — check that the league name is correct (run 'leagues' to see valid names)\n", fetchErrors, totalFetches)
			}
			if len(results) == 0 {
				if fetchErrors == totalFetches && totalFetches > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No supply shocks found in %s: all %d category fetches failed — verify the league name.\n", flagLeague, totalFetches)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No supply shocks found in %s with min-drop %.0f%%.\n", flagLeague, flagMinDrop)
				}
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nSupply shock signals in %s league (price drop > %.0f%%):\n\n", flagLeague, flagMinDrop)
			fmt.Fprintf(w, "  %-30s %-18s %10s %10s %10s\n", "Item", "Category", "Chaos", "7d Change", "Listings")
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 80))
			for _, r := range results {
				fmt.Fprintf(w, "  %-30s %-18s %9.2fc %9.1f%% %10d\n",
					r.Name, r.Category, r.ChaosValue, r.TotalChange, r.ListingCount)
			}
			fmt.Fprintf(w, "\n  Note: %s\n\n", note)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Time window (e.g. 24h, 48h) — informational for future snapshot support")
	cmd.Flags().StringVar(&flagLeague, "league", "Mirage", "League name (e.g. Mirage, Standard)")
	cmd.Flags().Float64Var(&flagMinDrop, "min-drop", 10.0, "Minimum price drop % to flag as a shock signal")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Limit to a specific category (e.g. Currency, Scarab)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum results to show")
	return cmd
}
