package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type flipResult struct {
	From        string  `json:"currency_from"`
	To          string  `json:"currency_to"`
	BuyAt       float64 `json:"buy_at_chaos"`
	SellAt      float64 `json:"sell_at_chaos"`
	ProfitChaos float64 `json:"profit_chaos"`
	ProfitPct   float64 `json:"profit_pct"`
	Budget      float64 `json:"budget_chaos"`
	CanAfford   int     `json:"can_afford_units"`
	Note        string  `json:"note,omitempty"`
}

type flipView struct {
	League  string       `json:"league"`
	Budget  float64      `json:"budget_chaos"`
	Results []flipResult `json:"results"`
}

func newNovelFlipCmd(flags *rootFlags) *cobra.Command {
	var flagBudget float64
	var flagLeague string
	var flagLimit int
	var flagMinProfit float64

	cmd := &cobra.Command{
		Use:   "flip",
		Short: "Given a chaos budget, find the single most profitable currency-to-currency round-trip trade.",
		Long:  "Fetches all currency exchange rates from poe.ninja and computes which currency-to-currency trade yields the highest profit for your budget. Accounts for bulk minimums implicitly (uses chaosEquivalent for pricing). Chaos Orb is the base currency and cannot be traded against itself.",
		Example: strings.Trim(`
  poe-ninja-pp-cli flip --budget 500 --league Mirage
  poe-ninja-pp-cli flip --budget 1000 --league Standard --min-profit 10 --json
  poe-ninja-pp-cli flip --budget 500 --league Mirage --limit 5 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would find best flip for %.0fc budget in %s\n", flagBudget, flagLeague)
				return nil
			}
			if flagBudget <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--budget must be a positive number of chaos orbs"))
			}
			if flagLeague == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required"))
			}

			cr, err := poeFetchCurrency(flagLeague, "Currency")
			if err != nil {
				return apiErr(fmt.Errorf("fetching currency data: %w", err))
			}

			// Build price map: name → chaosEquivalent
			prices := map[string]float64{}
			for _, l := range cr.Lines {
				if l.ChaosEquivalent > 0 {
					prices[l.CurrencyTypeName] = l.ChaosEquivalent
				}
			}

			// "Chaos Orb" is the base = 1.0 chaos
			prices["Chaos Orb"] = 1.0

			// For each pair A→B: buy B with chaos, get rate = chaosEquivalent(B)
			// If we start with flagBudget chaos, buy as many B as we can at price B,
			// then sell back to chaos. Simple round-trip: profit = budget * (1/priceB * priceB - 1)
			// which is always 0 for a perfect market. The actual flip is: buy B (get B units),
			// then sell B back at the implied rate. Real profit comes from spread.
			// We model: buyRate(B in chaos) = chaosEquivalent(B), sellRate(B for chaos) = chaosEquivalent(B)
			// For a direct flip: start with X chaos → buy Y of currency B → sell Y back = X chaos (no profit)
			// The actual opportunity is: buy B cheaply (when its chaos value > what you pay),
			// or sell B expensively. We use the receive/pay spread.
			// Simplified: profit comes from buying B at (chaosEquivalent / receive.value) and selling at (chaosEquivalent).

			var results []flipResult
			for _, l := range cr.Lines {
				if l.CurrencyTypeName == "Chaos Orb" || l.ChaosEquivalent <= 0 {
					continue
				}
				// Buy B at the implied receive rate (chaos you spend per B)
				// Sell B at the implied pay rate (chaos you get per B)
				// If receive.value > 0: cost to buy 1 B = 1/receive.value chaos
				// If pay.value > 0: revenue from selling 1 B = pay.value chaos (per chaos; actual rate = 1/pay_currency_id rate)
				// Simplification: use chaosEquivalent as sell price
				// and if there's a receive listing, use 1/receive.value as cost

				cost := l.ChaosEquivalent
				sell := l.ChaosEquivalent
				if l.Receive.Value > 0 {
					cost = 1.0 / l.Receive.Value
				}
				if l.Pay.Value > 0 {
					sell = l.Pay.Value
				}
				if cost <= 0 || sell <= 0 {
					continue
				}

				units := int(flagBudget / cost)
				if units <= 0 {
					continue
				}
				revenue := float64(units) * sell
				profit := revenue - float64(units)*cost
				profitPct := 0.0
				if cost > 0 {
					profitPct = (sell - cost) / cost * 100
				}

				if profit < flagMinProfit {
					continue
				}

				note := ""
				if l.ListingCount < 10 {
					note = fmt.Sprintf("low supply (%d listings)", l.ListingCount)
				}

				results = append(results, flipResult{
					From:        "Chaos Orb",
					To:          l.CurrencyTypeName,
					BuyAt:       cost,
					SellAt:      sell,
					ProfitChaos: profit,
					ProfitPct:   profitPct,
					Budget:      flagBudget,
					CanAfford:   units,
					Note:        note,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].ProfitChaos > results[j].ProfitChaos
			})
			if flagLimit > 0 && len(results) > flagLimit {
				results = results[:flagLimit]
			}

			view := flipView{League: flagLeague, Budget: flagBudget, Results: results}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No profitable flips found with the given budget and min-profit threshold.")
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nBest currency flips with %.0f chaos in %s league:\n\n", flagBudget, flagLeague)
			fmt.Fprintf(w, "  %-28s %10s %10s %12s %8s\n", "Currency", "Buy@", "Sell@", "Profit(c)", "Gain%")
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 72))
			for _, r := range results {
				note := ""
				if r.Note != "" {
					note = " [" + r.Note + "]"
				}
				fmt.Fprintf(w, "  %-28s %9.3fc %9.3fc %11.2fc %7.1f%%%s\n",
					r.To, r.BuyAt, r.SellAt, r.ProfitChaos, r.ProfitPct, note)
			}
			fmt.Fprintln(w)
			return nil
		},
	}
	cmd.Flags().Float64Var(&flagBudget, "budget", 0, "Budget in chaos orbs (required)")
	cmd.Flags().StringVar(&flagLeague, "league", "Mirage", "League name (e.g. Mirage, Standard)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum results to show")
	cmd.Flags().Float64Var(&flagMinProfit, "min-profit", 0, "Minimum profit in chaos to include a result")
	_ = strconv.Itoa(0) // suppress unused import if needed
	return cmd
}
