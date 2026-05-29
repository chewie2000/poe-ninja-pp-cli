package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

type velocityEntry struct {
	Category       string  `json:"category"`
	AvgChaos       float64 `json:"avg_chaos_value"`
	VelocityCPD    float64 `json:"velocity_cd_per_day"`
	AccelCPD2      float64 `json:"accel_cd_per_day2"`
	TotalChangePct float64 `json:"total_change_pct"`
	Signal         string  `json:"signal"`
	ItemCount      int     `json:"item_count"`
}

type velocityView struct {
	League  string          `json:"league"`
	Results []velocityEntry `json:"results"`
}

func newNovelVelocityCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagGame string
	var flagTop int

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Rank item categories by price acceleration computed from 7-day sparkline data across the entire database.",
		Long:  "Fetches all item categories from poe.ninja, computes the average price velocity (chaos/day) and acceleration (change in velocity) for each. HEATING = prices accelerating upward. COOLING = decelerating. STABLE = minimal movement.",
		Example: strings.Trim(`
  poe-ninja-pp-cli velocity --league Mirage
  poe-ninja-pp-cli velocity --league Mirage --top 10 --json
  poe-ninja-pp-cli velocity --league Standard --agent --select category,signal,velocity_cd_per_day`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if flagLeague == "" {
				flagLeague = poeDefaultLeague(flagGame)
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compute price velocity for all categories in %s %s\n", flagGame, flagLeague)
				return nil
			}
			if flagLeague == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required"))
			}

			// Fetch all item types in parallel
			type catResult struct {
				cat   string
				lines []poeItemLine
				err   error
			}
			ch := make(chan catResult, len(poeAllItemTypes)+2)
			var wg sync.WaitGroup

			fetchTypes := append(poeAllItemTypes, "Currency", "Fragment")
			for _, t := range fetchTypes {
				wg.Add(1)
				go func(cat string) {
					defer wg.Done()
					if cat == "Currency" || cat == "Fragment" {
						cr, err := poeFetchCurrencyGame(flagLeague, cat, flagGame)
						if err != nil {
							ch <- catResult{cat: cat, err: err}
							return
						}
						// Convert currency lines to item-like structure
						var lines []poeItemLine
						for _, l := range cr.Lines {
							lines = append(lines, poeItemLine{
								Name:         l.CurrencyTypeName,
								ChaosValue:   l.ChaosEquivalent,
								ListingCount: l.ListingCount,
								SparkLine:    l.ReceiveSparkLine,
							})
						}
						ch <- catResult{cat: cat, lines: lines}
					} else {
						ir, err := poeFetchItemGame(flagLeague, cat, flagGame)
						if err != nil {
							ch <- catResult{cat: cat, err: err}
							return
						}
						ch <- catResult{cat: cat, lines: ir.Lines}
					}
				}(t)
			}
			go func() { wg.Wait(); close(ch) }()

			var entries []velocityEntry
			var fetchErrors, totalFetches int
			for r := range ch {
				totalFetches++
				if r.err != nil || len(r.lines) == 0 {
					if r.err != nil {
						fetchErrors++
					}
					continue
				}
				// Aggregate velocity across all items in this category
				var totalVel, totalAccel, totalChange, totalChaos float64
				count := 0
				for _, l := range r.lines {
					if l.ChaosValue <= 0 {
						continue
					}
					vel, accel := sparklineVelocity(l.SparkLine.Data)
					totalVel += vel
					totalAccel += accel
					totalChange += l.SparkLine.TotalChange
					totalChaos += l.ChaosValue
					count++
				}
				if count == 0 {
					continue
				}
				avgVel := totalVel / float64(count)
				avgAccel := totalAccel / float64(count)
				avgChange := totalChange / float64(count)
				avgChaos := totalChaos / float64(count)

				signal := "STABLE"
				if avgAccel > 0.5 || (avgVel > 0.5 && avgChange > 5) {
					signal = "HEATING"
				} else if avgAccel < -0.5 || (avgVel < -0.5 && avgChange < -5) {
					signal = "COOLING"
				}

				entries = append(entries, velocityEntry{
					Category:       r.cat,
					AvgChaos:       avgChaos,
					VelocityCPD:    avgVel,
					AccelCPD2:      avgAccel,
					TotalChangePct: avgChange,
					Signal:         signal,
					ItemCount:      count,
				})
			}

			sort.Slice(entries, func(i, j int) bool {
				return entries[i].AccelCPD2 > entries[j].AccelCPD2
			})
			if flagTop > 0 && len(entries) > flagTop {
				entries = entries[:flagTop]
			}

			if fetchErrors > 0 && totalFetches > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %d of %d categories failed to fetch — check that the league name is correct (run 'leagues' to see valid names)\n", fetchErrors, totalFetches)
			}

			view := velocityView{League: flagLeague, Results: entries}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nEconomy velocity in %s league:\n\n", flagLeague)
			fmt.Fprintf(w, "  %-22s %8s %10s %10s %10s\n", "Category", "Signal", "Avg Chaos", "Vel(c/d)", "Accel")
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 65))
			for _, e := range entries {
				fmt.Fprintf(w, "  %-22s %8s %9.2fc %9.2f %10.2f\n",
					e.Category, e.Signal, e.AvgChaos, e.VelocityCPD, e.AccelCPD2)
			}
			fmt.Fprintln(w)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagGame, "game", "poe2", "Game version: poe1 or poe2")
	cmd.Flags().StringVar(&flagLeague, "league", "", "League name (default: Standard for poe2, Mirage for poe1)")
	cmd.Flags().IntVar(&flagTop, "top", 20, "Number of categories to show (0 = all)")
	return cmd
}
