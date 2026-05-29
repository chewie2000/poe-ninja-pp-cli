package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type trendView struct {
	Name        string    `json:"name"`
	League      string    `json:"league"`
	ChaosValue  float64   `json:"chaos_value"`
	DivineValue float64   `json:"divine_value,omitempty"`
	TotalChange float64   `json:"total_change_pct"`
	Sparkline   []float64 `json:"sparkline_data"`
	Chart       string    `json:"chart"`
	Category    string    `json:"category"`
}

func newNovelTrendCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string

	cmd := &cobra.Command{
		Use:   "trend <item-name>",
		Short: "Render 7-day price history for any currency or item as an ASCII chart directly in the terminal.",
		Long:  "Render 7-day price history for any currency or item as an ASCII chart. Shows price trend, total change %, and sparkline visualization.",
		Example: strings.Trim(`
  poe-ninja-pp-cli trend "Divine Orb" --league Mirage
  poe-ninja-pp-cli trend "Mirror of Kalandra" --league Standard
  poe-ninja-pp-cli trend "Enlighten Support" --league Mirage --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch trend data for:", strings.Join(args, " "))
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("item name is required as positional argument"))
			}
			if flagLeague == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required (e.g. --league Mirage)"))
			}

			query := strings.ToLower(strings.Join(args, " "))

			// Try currency first
			if cr, err := poeFetchCurrency(flagLeague, "Currency"); err == nil {
				for _, line := range cr.Lines {
					if strings.EqualFold(line.CurrencyTypeName, strings.Join(args, " ")) ||
						strings.Contains(strings.ToLower(line.CurrencyTypeName), query) {
						v := trendView{
							Name:        line.CurrencyTypeName,
							League:      flagLeague,
							ChaosValue:  line.ChaosEquivalent,
							TotalChange: line.ReceiveSparkLine.TotalChange,
							Sparkline:   line.ReceiveSparkLine.Data,
							Chart:       sparklineASCII(line.ReceiveSparkLine.Data),
							Category:    "Currency",
						}
						return renderTrend(cmd, flags, v)
					}
				}
			}

			// Try common item types
			for _, t := range []string{"UniqueWeapon", "UniqueArmour", "UniqueJewel", "SkillGem", "DivinationCard", "Scarab", "Oil", "Essence"} {
				ir, err := poeFetchItem(flagLeague, t)
				if err != nil {
					continue
				}
				for _, line := range ir.Lines {
					if strings.EqualFold(line.Name, strings.Join(args, " ")) ||
						strings.Contains(strings.ToLower(line.Name), query) {
						v := trendView{
							Name:        line.Name,
							League:      flagLeague,
							ChaosValue:  line.ChaosValue,
							DivineValue: line.DivineValue,
							TotalChange: line.SparkLine.TotalChange,
							Sparkline:   line.SparkLine.Data,
							Chart:       sparklineASCII(line.SparkLine.Data),
							Category:    t,
						}
						return renderTrend(cmd, flags, v)
					}
				}
			}

			return notFoundErr(fmt.Errorf("item %q not found in league %q — try the exact name from poe.ninja", strings.Join(args, " "), flagLeague))
		},
	}
	cmd.Flags().StringVar(&flagLeague, "league", "Mirage", "League name (e.g. Mirage, Standard, Hardcore Mirage)")
	return cmd
}

func renderTrend(cmd *cobra.Command, flags *rootFlags, v trendView) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}

	w := cmd.OutOrStdout()
	changeSign := "+"
	if v.TotalChange < 0 {
		changeSign = ""
	}
	fmt.Fprintf(w, "\n%s [%s] — %s league\n", bold(v.Name), v.Category, v.League)
	fmt.Fprintf(w, "  Price:  %.2f chaos", v.ChaosValue)
	if v.DivineValue > 0 {
		fmt.Fprintf(w, " (%.2f divine)", v.DivineValue)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  7-day:  %s%.1f%%\n", changeSign, v.TotalChange)
	fmt.Fprintf(w, "  Chart:  %s\n", v.Chart)
	if len(v.Sparkline) > 0 {
		fmt.Fprintf(w, "  Points: %d days of history\n", len(v.Sparkline))
	}
	fmt.Fprintln(w)
	return nil
}
