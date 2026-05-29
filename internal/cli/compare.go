package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

type compareEntry struct {
	Name       string  `json:"name"`
	Category   string  `json:"category"`
	League1    string  `json:"league1"`
	League2    string  `json:"league2"`
	ChaosL1    float64 `json:"chaos_league1"`
	ChaosL2    float64 `json:"chaos_league2"`
	DeltaChaos float64 `json:"delta_chaos"`
	DeltaPct   float64 `json:"delta_pct"`
	CheaperIn  string  `json:"cheaper_in"`
}

type compareView struct {
	Query   string         `json:"query"`
	Results []compareEntry `json:"results"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagLeague1 string
	var flagLeague2 string

	cmd := &cobra.Command{
		Use:   "compare <item-name>",
		Short: "Show the price of the same item side-by-side in two leagues to identify value arbitrage opportunities.",
		Long:  "Fetch the price of a currency or item in two leagues simultaneously and display the difference. Useful for finding items cheaper in Standard to buy and use in the challenge league.",
		Example: strings.Trim(`
  poe-ninja-pp-cli compare "Divine Orb" --league1 Mirage --league2 Standard
  poe-ninja-pp-cli compare "Mirror of Kalandra" --league1 Mirage --league2 Standard --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compare %q between %s and %s\n", strings.Join(args, " "), flagLeague1, flagLeague2)
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("item name is required as positional argument"))
			}
			if flagLeague1 == "" || flagLeague2 == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league1 and --league2 are required"))
			}

			query := strings.Join(args, " ")
			qLower := strings.ToLower(query)

			// Fetch currency data from both leagues in parallel
			type currResult struct {
				resp *poeCurrencyResponse
				err  error
			}
			var wg sync.WaitGroup
			cr1ch := make(chan currResult, 1)
			cr2ch := make(chan currResult, 1)

			wg.Add(2)
			go func() {
				defer wg.Done()
				r, e := poeFetchCurrency(flagLeague1, "Currency")
				cr1ch <- currResult{r, e}
			}()
			go func() {
				defer wg.Done()
				r, e := poeFetchCurrency(flagLeague2, "Currency")
				cr2ch <- currResult{r, e}
			}()
			wg.Wait()
			cr1 := <-cr1ch
			cr2 := <-cr2ch

			view := compareView{Query: query}

			// Match currency
			if cr1.err == nil && cr2.err == nil {
				prices1 := map[string]float64{}
				for _, l := range cr1.resp.Lines {
					prices1[strings.ToLower(l.CurrencyTypeName)] = l.ChaosEquivalent
				}
				for _, l := range cr2.resp.Lines {
					name := strings.ToLower(l.CurrencyTypeName)
					if strings.Contains(name, qLower) || strings.EqualFold(l.CurrencyTypeName, query) {
						if p1, ok := prices1[name]; ok {
							delta := l.ChaosEquivalent - p1
							deltaPct := 0.0
							if p1 > 0 {
								deltaPct = (l.ChaosEquivalent - p1) / p1 * 100
							}
							cheaper := flagLeague1
							if l.ChaosEquivalent < p1 {
								cheaper = flagLeague2
							}
							view.Results = append(view.Results, compareEntry{
								Name:       l.CurrencyTypeName,
								Category:   "Currency",
								League1:    flagLeague1,
								League2:    flagLeague2,
								ChaosL1:    p1,
								ChaosL2:    l.ChaosEquivalent,
								DeltaChaos: delta,
								DeltaPct:   deltaPct,
								CheaperIn:  cheaper,
							})
						}
					}
				}
			}

			if len(view.Results) == 0 {
				return notFoundErr(fmt.Errorf("%q not found in currency data — check spelling or league names", query))
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nComparing %q: %s vs %s\n\n", query, flagLeague1, flagLeague2)
			fmt.Fprintf(w, "  %-30s %12s %12s %10s %15s\n", "Item", flagLeague1, flagLeague2, "Delta", "Cheaper In")
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 82))
			for _, r := range view.Results {
				sign := "+"
				if r.DeltaChaos < 0 {
					sign = ""
				}
				fmt.Fprintf(w, "  %-30s %11.2fc %11.2fc %s%.1f%% %15s\n",
					r.Name, r.ChaosL1, r.ChaosL2, sign, r.DeltaPct, r.CheaperIn)
			}
			fmt.Fprintln(w)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagLeague1, "league1", "Mirage", "First league name (e.g. Mirage)")
	cmd.Flags().StringVar(&flagLeague2, "league2", "Standard", "Second league name (e.g. Standard)")
	return cmd
}
