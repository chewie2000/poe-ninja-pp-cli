package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type portfolioEntry struct {
	Name        string  `json:"name"`
	Quantity    float64 `json:"quantity"`
	ChaosEach   float64 `json:"chaos_each"`
	TotalChaos  float64 `json:"total_chaos"`
	TrendChange float64 `json:"trend_7d_pct"`
	Chart       string  `json:"chart"`
}

type portfolioView struct {
	League      string           `json:"league"`
	Holdings    []portfolioEntry `json:"holdings"`
	TotalChaos  float64          `json:"total_chaos"`
	DivineChaos float64          `json:"divine_chaos_rate,omitempty"`
	TotalDivine float64          `json:"total_divine,omitempty"`
}

func newNovelPortfolioCmd(flags *rootFlags) *cobra.Command {
	var flagItems string
	var flagLeague string

	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Track a user-defined list of currencies and items by total chaos value, updated each sync.",
		Long:  "Like networth, but focused on currencies with trend data shown for each holding. Takes a comma-separated 'Name:qty' list and shows current price, total value, and 7-day trend chart for each holding.",
		Example: strings.Trim(`
  poe-ninja-pp-cli portfolio --items "Divine Orb:50,Chaos Orb:1000" --league Mirage
  poe-ninja-pp-cli portfolio --items "Mirror of Kalandra:1,Exalted Orb:5" --league Standard --json
  poe-ninja-pp-cli portfolio --items "Divine Orb:10" --league Mirage --agent --select name,total_chaos,trend_7d_pct`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would calculate portfolio value for supplied items")
				return nil
			}
			if flagItems == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--items is required (e.g. --items \"Divine Orb:50,Chaos Orb:1000\")"))
			}
			if flagLeague == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required"))
			}

			inventory := parseItemsFlag(flagItems)

			cr, err := poeFetchCurrency(flagLeague, "Currency")
			if err != nil {
				return apiErr(fmt.Errorf("fetching currency data: %w", err))
			}

			currencyData := map[string]poeCurrencyLine{}
			for _, l := range cr.Lines {
				currencyData[strings.ToLower(l.CurrencyTypeName)] = l
			}
			currencyData["chaos orb"] = poeCurrencyLine{
				CurrencyTypeName: "Chaos Orb",
				ChaosEquivalent:  1.0,
			}

			divinePrice := 0.0
			if d, ok := currencyData["divine orb"]; ok {
				divinePrice = d.ChaosEquivalent
			}

			var holdings []portfolioEntry
			totalChaos := 0.0

			for name, qty := range inventory {
				lKey := strings.ToLower(name)
				if l, ok := currencyData[lKey]; ok {
					total := l.ChaosEquivalent * qty
					totalChaos += total
					holdings = append(holdings, portfolioEntry{
						Name:        l.CurrencyTypeName,
						Quantity:    qty,
						ChaosEach:   l.ChaosEquivalent,
						TotalChaos:  total,
						TrendChange: l.ReceiveSparkLine.TotalChange,
						Chart:       sparklineASCII(l.ReceiveSparkLine.Data),
					})
				} else {
					holdings = append(holdings, portfolioEntry{
						Name:        name,
						Quantity:    qty,
						TrendChange: 0,
						Chart:       "not found",
					})
				}
			}

			sort.Slice(holdings, func(i, j int) bool {
				return holdings[i].TotalChaos > holdings[j].TotalChaos
			})

			view := portfolioView{
				League:      flagLeague,
				Holdings:    holdings,
				TotalChaos:  totalChaos,
				DivineChaos: divinePrice,
			}
			if divinePrice > 0 {
				view.TotalDivine = totalChaos / divinePrice
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nPortfolio — %s league\n\n", flagLeague)
			fmt.Fprintf(w, "  %-28s %8s %12s %12s %10s  %s\n", "Currency", "Qty", "Each (c)", "Total (c)", "7d Change", "Trend")
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 85))
			for _, h := range holdings {
				sign := "+"
				if h.TrendChange < 0 {
					sign = ""
				}
				fmt.Fprintf(w, "  %-28s %8.0f %11.2fc %11.2fc %9s%.1f%%  %s\n",
					h.Name, h.Quantity, h.ChaosEach, h.TotalChaos, sign, h.TrendChange, h.Chart)
			}
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 85))
			fmt.Fprintf(w, "  %-61s %11.2fc\n", "TOTAL", totalChaos)
			if divinePrice > 0 {
				fmt.Fprintf(w, "  %-61s %11.2f divine\n", "TOTAL (divine)", totalChaos/divinePrice)
			}
			fmt.Fprintln(w)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagItems, "items", "", "Comma-separated currency:quantity pairs (e.g. \"Divine Orb:50,Chaos Orb:1000\")")
	cmd.Flags().StringVar(&flagLeague, "league", "Mirage", "League name (e.g. Mirage, Standard)")
	return cmd
}
