package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type networthHolding struct {
	Name         string  `json:"name"`
	Quantity     float64 `json:"quantity"`
	ChaosEach    float64 `json:"chaos_each"`
	TotalChaos   float64 `json:"total_chaos"`
	ListingCount int     `json:"listing_count"`
	Liquid       bool    `json:"liquid"`
	Note         string  `json:"note,omitempty"`
}

type networthView struct {
	League        string            `json:"league"`
	Holdings      []networthHolding `json:"holdings"`
	TotalChaos    float64           `json:"total_chaos"`
	LiquidChaos   float64           `json:"liquid_chaos"`
	IlliquidChaos float64           `json:"illiquid_chaos"`
	DivineChaos   float64           `json:"divine_chaos_rate"`
	TotalDivine   float64           `json:"total_divine,omitempty"`
}

// parseItemsFlag parses "ItemA:qty,ItemB:qty" or "ItemA:qty ItemB:qty" into map.
func parseItemsFlag(s string) map[string]float64 {
	result := map[string]float64{}
	// Support both comma and space separators
	s = strings.ReplaceAll(s, ", ", ",")
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' })
	for _, part := range parts {
		part = strings.TrimSpace(part)
		idx := strings.LastIndex(part, ":")
		if idx < 0 {
			result[part] = 1
			continue
		}
		name := strings.TrimSpace(part[:idx])
		qty, err := strconv.ParseFloat(strings.TrimSpace(part[idx+1:]), 64)
		if err != nil {
			qty = 1
		}
		result[name] = qty
	}
	return result
}

func newNovelNetworthCmd(flags *rootFlags) *cobra.Command {
	var flagItems string
	var flagLeague string
	var flagGame string
	var flagFormat string
	var flagLiquidThreshold int

	cmd := &cobra.Command{
		Use:   "networth",
		Short: "Calculate total chaos value of a user-supplied inventory, flagging illiquid holdings (low listing count) vs. paper wealth.",
		Long:  "Takes a comma-separated list of 'ItemName:quantity' pairs and calculates total chaos value using live poe.ninja prices. Holdings with fewer than --liquid-threshold listings are flagged as illiquid (hard to sell quickly).",
		Example: strings.Trim(`
  poe-ninja-pp-cli networth --items "Divine Orb:50,Mirror of Kalandra:1" --league Mirage
  poe-ninja-pp-cli networth --items "Chaos Orb:1000,Exalted Orb:10" --league Standard --format divine
  poe-ninja-pp-cli networth --items "Divine Orb:10,Awakened Empower Support:2" --league Mirage --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if flagLeague == "" {
				flagLeague = poeDefaultLeague(flagGame)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would calculate net worth for supplied inventory")
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
			if len(inventory) == 0 {
				return usageErr(fmt.Errorf("could not parse --items; use format \"Divine Orb:50,Chaos Orb:100\""))
			}

			// Fetch currency data
			cr, err := poeFetchCurrencyGame(flagLeague, "Currency", flagGame)
			if err != nil {
				return apiErr(fmt.Errorf("fetching currency data: %w", err))
			}

			// Build lookup maps
			currencyPrice := map[string]float64{"Chaos Orb": 1.0}
			currencyListing := map[string]int{}
			for _, l := range cr.Lines {
				if l.ChaosEquivalent > 0 {
					currencyPrice[l.CurrencyTypeName] = l.ChaosEquivalent
				}
				currencyListing[l.CurrencyTypeName] = l.ListingCount
			}
			divinePrice := currencyPrice["Divine Orb"]

			var holdings []networthHolding
			var totalChaos, liquidChaos, illiquidChaos float64

			for name, qty := range inventory {
				chaos := 0.0
				listing := 0
				note := ""

				// Try currency first
				if p, ok := currencyPrice[name]; ok {
					chaos = p
					listing = currencyListing[name]
				} else {
					// Try items - search first common categories
					found := false
					for _, t := range []string{"UniqueWeapon", "UniqueArmour", "UniqueJewel", "SkillGem", "UniqueFlask", "UniqueAccessory"} {
						ir, ferr := poeFetchItem(flagLeague, t)
						if ferr != nil {
							continue
						}
						for _, l := range ir.Lines {
							if strings.EqualFold(l.Name, name) {
								chaos = l.ChaosValue
								listing = l.ListingCount
								found = true
								break
							}
						}
						if found {
							break
						}
					}
					if !found {
						note = "not found in poe.ninja — using 0c"
					}
				}

				liquid := listing >= flagLiquidThreshold
				if listing == 0 && chaos > 0 {
					liquid = false
					note = "no listing count data"
				}

				totalValue := chaos * qty
				totalChaos += totalValue
				if liquid {
					liquidChaos += totalValue
				} else {
					illiquidChaos += totalValue
					if note == "" {
						note = fmt.Sprintf("illiquid (%d listings)", listing)
					}
				}

				holdings = append(holdings, networthHolding{
					Name:         name,
					Quantity:     qty,
					ChaosEach:    chaos,
					TotalChaos:   totalValue,
					ListingCount: listing,
					Liquid:       liquid,
					Note:         note,
				})
			}

			sort.Slice(holdings, func(i, j int) bool {
				return holdings[i].TotalChaos > holdings[j].TotalChaos
			})

			view := networthView{
				League:        flagLeague,
				Holdings:      holdings,
				TotalChaos:    totalChaos,
				LiquidChaos:   liquidChaos,
				IlliquidChaos: illiquidChaos,
				DivineChaos:   divinePrice,
			}
			if divinePrice > 0 && flagFormat == "divine" {
				view.TotalDivine = totalChaos / divinePrice
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\nNet Worth — %s league\n\n", flagLeague)
			fmt.Fprintf(w, "  %-30s %8s %12s %12s %8s\n", "Item", "Qty", "Each (c)", "Total (c)", "Liquid?")
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 74))
			for _, h := range holdings {
				liq := "✓"
				if !h.Liquid {
					liq = "✗"
				}
				note := ""
				if h.Note != "" {
					note = " — " + h.Note
				}
				fmt.Fprintf(w, "  %-30s %8.0f %11.2fc %11.2fc %8s%s\n",
					h.Name, h.Quantity, h.ChaosEach, h.TotalChaos, liq, note)
			}
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 74))
			fmt.Fprintf(w, "  %-51s %11.2fc\n", "TOTAL", totalChaos)
			if divinePrice > 0 {
				fmt.Fprintf(w, "  %-51s %11.2f divine\n", "TOTAL (divine)", totalChaos/divinePrice)
			}
			fmt.Fprintf(w, "  %-51s %11.2fc (liquid)\n", "LIQUID", liquidChaos)
			fmt.Fprintf(w, "  %-51s %11.2fc (illiquid)\n", "ILLIQUID", illiquidChaos)
			fmt.Fprintln(w)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagItems, "items", "", "Comma-separated item:quantity pairs (e.g. \"Divine Orb:50,Mirror of Kalandra:1\")")
	cmd.Flags().StringVar(&flagGame, "game", "poe2", "Game version: poe1 or poe2")
	cmd.Flags().StringVar(&flagLeague, "league", "", "League name (default: Standard for poe2, Mirage for poe1)")
	cmd.Flags().StringVar(&flagFormat, "format", "chaos", "Output format: chaos or divine")
	cmd.Flags().IntVar(&flagLiquidThreshold, "liquid-threshold", 10, "Minimum listings to consider an item liquid")
	return cmd
}
