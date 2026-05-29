package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type arbCycle struct {
	Path       []string `json:"path"`
	GainPct    float64  `json:"gain_pct"`
	GainFactor float64  `json:"gain_factor"`
	Hops       int      `json:"hops"`
}

type arbView struct {
	League string     `json:"league"`
	Cycles []arbCycle `json:"cycles"`
}

// arbFindCycles uses Bellman-Ford on the currency graph to find profitable arbitrage cycles.
// Edge weight = -log(rate). Negative cycle = profitable arbitrage.
// rate(A→B) = chaosEquivalent(B) / chaosEquivalent(A)
func arbFindCycles(prices map[string]float64, minGainPct float64, maxHops int) []arbCycle {
	names := make([]string, 0, len(prices))
	for n := range prices {
		names = append(names, n)
	}
	sort.Strings(names)

	idx := map[string]int{}
	for i, n := range names {
		idx[n] = i
	}
	n := len(names)

	// Build edges: for each pair, edge weight = -log(rate(i→j))
	type edge struct {
		u, v int
		w    float64
	}
	var edges []edge
	for i, a := range names {
		for j, b := range names {
			if i == j || prices[a] <= 0 || prices[b] <= 0 {
				continue
			}
			rate := prices[b] / prices[a]
			if rate <= 0 {
				continue
			}
			w := -math.Log(rate)
			edges = append(edges, edge{i, j, w})
		}
	}

	// Bellman-Ford from each starting node to detect negative cycles reachable from it.
	// We run up to maxHops iterations and track paths.
	type state struct {
		dist []float64
		prev []int
	}

	var cycles []arbCycle
	seen := map[string]bool{}

	for start := 0; start < n; start++ {
		dist := make([]float64, n)
		prev := make([]int, n)
		for i := range dist {
			dist[i] = math.Inf(1)
			prev[i] = -1
		}
		dist[start] = 0

		// Relax edges up to maxHops times
		for iter := 0; iter < maxHops; iter++ {
			updated := false
			for _, e := range edges {
				if dist[e.u]+e.w < dist[e.v] {
					dist[e.v] = dist[e.u] + e.w
					prev[e.v] = e.u
					updated = true
				}
			}
			if !updated {
				break
			}
		}

		// Check for one more relaxation — if possible, negative cycle exists
		for _, e := range edges {
			if dist[e.u]+e.w < dist[e.v]-1e-9 {
				// Trace back the cycle
				path := []int{}
				visited := make([]bool, n)
				cur := e.v
				for !visited[cur] {
					visited[cur] = true
					path = append(path, cur)
					if prev[cur] < 0 {
						break
					}
					cur = prev[cur]
				}
				// cur is now the start of the cycle
				cycleStart := cur
				cycleNodes := []int{cycleStart}
				for _, node := range path {
					cycleNodes = append(cycleNodes, node)
					if node == cycleStart && len(cycleNodes) > 1 {
						break
					}
				}

				// Compute actual gain
				gainFactor := 1.0
				cycleNames := []string{}
				for i := 0; i < len(cycleNodes)-1; i++ {
					a := names[cycleNodes[i]]
					b := names[cycleNodes[i+1]]
					if prices[a] > 0 && prices[b] > 0 {
						gainFactor *= prices[b] / prices[a]
					}
					cycleNames = append(cycleNames, a)
				}
				if len(cycleNames) > 0 {
					cycleNames = append(cycleNames, cycleNames[0])
				}

				gainPct := (gainFactor - 1) * 100
				if gainPct < minGainPct || len(cycleNames) < 3 {
					continue
				}

				key := strings.Join(cycleNames, "→")
				if seen[key] {
					continue
				}
				seen[key] = true

				cycles = append(cycles, arbCycle{
					Path:       cycleNames,
					GainPct:    gainPct,
					GainFactor: gainFactor,
					Hops:       len(cycleNames) - 1,
				})
			}
		}
	}

	sort.Slice(cycles, func(i, j int) bool {
		return cycles[i].GainPct > cycles[j].GainPct
	})
	return cycles
}

func newNovelArbCmd(flags *rootFlags) *cobra.Command {
	var flagBudget float64
	var flagLeague string
	var flagGame string
	var flagMinGain float64
	var flagLimit int
	var flagMaxHops int

	cmd := &cobra.Command{
		Use:   "arb",
		Short: "Find profitable multi-hop currency trading cycles using graph math over all exchange rates simultaneously.",
		Long:  "Runs Bellman-Ford on the full currency graph to find profitable trading cycles. A cycle is A→B→C→A where the chain of trades yields more chaos than you started with. Requires a network connection; prices are fetched live from poe.ninja.",
		Example: strings.Trim(`
  poe-ninja-pp-cli arb --game poe2 --league Standard
  poe-ninja-pp-cli arb --game poe2 --league Standard --min-gain 2 --budget 500
  poe-ninja-pp-cli arb --game poe1 --league Mirage --max-hops 3 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if flagLeague == "" {
				flagLeague = poeDefaultLeague(flagGame)
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would find arbitrage cycles in %s %s league\n", flagGame, flagLeague)
				return nil
			}

			cr, err := poeFetchCurrencyGame(flagLeague, "Currency", flagGame)
			if err != nil {
				return apiErr(fmt.Errorf("fetching currency data: %w", err))
			}

			// Build price map using chaosEquivalent
			prices := map[string]float64{"Chaos Orb": 1.0}
			for _, l := range cr.Lines {
				if l.ChaosEquivalent > 0 {
					prices[l.CurrencyTypeName] = l.ChaosEquivalent
				}
			}

			cycles := arbFindCycles(prices, flagMinGain, flagMaxHops)
			if flagLimit > 0 && len(cycles) > flagLimit {
				cycles = cycles[:flagLimit]
			}

			view := arbView{League: flagLeague, Cycles: cycles}

			if len(cycles) == 0 {
				msg := fmt.Sprintf("No arbitrage cycles found with min-gain %.1f%% in %s. The market may be efficient. Try --min-gain 0 to see all cycles.", flagMinGain, flagLeague)
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{"league": flagLeague, "cycles": []any{}, "note": msg})
				}
				fmt.Fprintln(cmd.OutOrStdout(), msg)
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if flagBudget > 0 {
					type arbWithBudget struct {
						arbView
						Budget          float64 `json:"budget_chaos"`
						EstimatedProfit float64 `json:"estimated_profit_chaos"`
					}
					bestGain := 0.0
					if len(cycles) > 0 {
						bestGain = cycles[0].GainPct
					}
					return enc.Encode(arbWithBudget{view, flagBudget, flagBudget * bestGain / 100})
				}
				return enc.Encode(view)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nArbitrage cycles in %s league (min gain: %.1f%%):\n\n", flagLeague, flagMinGain)
			for i, c := range cycles {
				budgetStr := ""
				if flagBudget > 0 {
					profit := flagBudget * c.GainPct / 100
					budgetStr = fmt.Sprintf(" → +%.1fc profit on %.0fc budget", profit, flagBudget)
				}
				fmt.Fprintf(w, "  #%d [+%.2f%%] %s%s\n", i+1, c.GainPct, strings.Join(c.Path, " → "), budgetStr)
			}
			fmt.Fprintln(w)
			return nil
		},
	}
	cmd.Flags().Float64Var(&flagBudget, "budget", 0, "Your chaos budget (for estimated profit calculation)")
	cmd.Flags().StringVar(&flagGame, "game", "poe2", "Game version: poe1 or poe2")
	cmd.Flags().StringVar(&flagLeague, "league", "", "League name (default: Standard for poe2, Mirage for poe1)")
	cmd.Flags().Float64Var(&flagMinGain, "min-gain", 1.0, "Minimum gain % to include a cycle (e.g. 2 = 2%)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum cycles to show")
	cmd.Flags().IntVar(&flagMaxHops, "max-hops", 4, "Maximum hops in a cycle (2-5 recommended)")
	return cmd
}
