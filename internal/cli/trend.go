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
	var flagGame string

	cmd := &cobra.Command{
		Use:   "trend <item-name>",
		Short: "Render 7-day price history for any currency or item as an ASCII chart directly in the terminal.",
		Long:  "Render 7-day price history for any currency or item as an ASCII chart. Shows price trend, total change %, and sparkline visualization.",
		Example: strings.Trim(`
  poe-ninja-pp-cli trend "Divine Orb" --game poe2
  poe-ninja-pp-cli trend "Divine Orb" --game poe1 --league Mirage
  poe-ninja-pp-cli trend "Mirror of Kalandra" --game poe1 --league Standard --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if flagLeague == "" {
				flagLeague = poeDefaultLeague(flagGame)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch trend data for:", strings.Join(args, " "))
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("item name is required as positional argument"))
			}

			query := strings.ToLower(strings.Join(args, " "))

			// Try currency first — surface the error if the API call itself fails
			cr, currErr := poeFetchCurrencyGame(flagLeague, "Currency", flagGame)
			if currErr != nil {
				return apiErr(fmt.Errorf("fetching %s currency data: %w", flagGame, currErr))
			}
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

			// Try common item types
			for _, t := range []string{"UniqueWeapon", "UniqueArmour", "UniqueJewel", "SkillGem", "DivinationCard", "Scarab", "Oil", "Essence"} {
				ir, err := poeFetchItemGame(flagLeague, t, flagGame)
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

			// Show a few available currencies to help the user
			var available []string
			for _, l := range cr.Lines {
				available = append(available, l.CurrencyTypeName)
				if len(available) >= 5 {
					break
				}
			}
			hint := ""
			if len(available) > 0 {
				hint = fmt.Sprintf(" Available currencies include: %s", strings.Join(available, ", "))
			}
			return notFoundErr(fmt.Errorf("item %q not found in %s %s league.%s", strings.Join(args, " "), flagGame, flagLeague, hint))
		},
	}
	cmd.Flags().StringVar(&flagGame, "game", "poe2", "Game version: poe1 or poe2")
	cmd.Flags().StringVar(&flagLeague, "league", "", "League name (default: Standard for poe2, Mirage for poe1)")
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
