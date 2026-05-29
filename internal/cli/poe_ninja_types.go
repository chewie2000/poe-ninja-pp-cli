package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// poeSparkLine holds 7-day price history data from poe.ninja.
type poeSparkLine struct {
	Data        []float64 `json:"data"`
	TotalChange float64   `json:"totalChange"`
}

// poeCurrencyLine is one entry from the poe.ninja currencyoverview response.
type poeCurrencyLine struct {
	CurrencyTypeName          string       `json:"currencyTypeName"`
	Pay                       poeExchange  `json:"pay"`
	Receive                   poeExchange  `json:"receive"`
	PaySparkLine              poeSparkLine `json:"paySparkLine"`
	ReceiveSparkLine          poeSparkLine `json:"receiveSparkLine"`
	ChaosEquivalent           float64      `json:"chaosEquivalent"`
	LowConfidencePaySparkLine poeSparkLine `json:"lowConfidencePaySparkLine"`
	DetailsID                 string       `json:"detailsId"`
	ListingCount              int          `json:"listingCount"`
}

type poeExchange struct {
	ID            int     `json:"id"`
	LeagueID      int     `json:"league_id"`
	PayCurrencyID int     `json:"pay_currency_id"`
	GetCurrencyID int     `json:"get_currency_id"`
	Value         float64 `json:"value"`
	ListingCount  int     `json:"listing_count"`
}

// poeCurrencyResponse is the full response from /api/data/currencyoverview.
type poeCurrencyResponse struct {
	Lines           []poeCurrencyLine `json:"lines"`
	CurrencyDetails []struct {
		ID      int    `json:"id"`
		Icon    string `json:"icon"`
		Name    string `json:"name"`
		TradeID string `json:"tradeId"`
	} `json:"currencyDetails"`
}

// poeItemLine is one entry from the poe.ninja itemoverview response.
type poeItemLine struct {
	ID            int          `json:"id"`
	Name          string       `json:"name"`
	Icon          string       `json:"icon"`
	MapTier       int          `json:"mapTier"`
	LevelRequired int          `json:"levelRequired"`
	StackSize     int          `json:"stackSize"`
	Variant       string       `json:"variant"`
	ProphecyText  string       `json:"prophecyText"`
	ArtFilename   string       `json:"artFilename"`
	Links         int          `json:"links"`
	ItemClass     int          `json:"itemClass"`
	SparkLine     poeSparkLine `json:"sparkline"`
	ChaosValue    float64      `json:"chaosValue"`
	DivineValue   float64      `json:"divineValue"`
	ExaltedValue  float64      `json:"exaltedValue"`
	Count         int          `json:"count"`
	DetailsID     string       `json:"detailsId"`
	Corrupted     bool         `json:"corrupted"`
	ListingCount  int          `json:"listingCount"`
	FlavourText   string       `json:"flavourText"`
}

// poeItemResponse is the full response from /api/data/itemoverview.
type poeItemResponse struct {
	Lines []poeItemLine `json:"lines"`
}

// poe2CurrencyLine is one entry from the PoE2 currency overview response.
// PoE2 uses primaryValue instead of chaosEquivalent, and different sparkline fields.
type poe2CurrencyLine struct {
	CurrencyTypeName string       `json:"currencyTypeName"`
	PrimaryValue     float64      `json:"primaryValue"`
	SparkLine        poeSparkLine `json:"sparkline"`
	ListingCount     int          `json:"listingCount"`
	DetailsID        string       `json:"detailsId"`
}

type poe2CurrencyResponse struct {
	Lines []poe2CurrencyLine `json:"lines"`
	Items []struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Icon    string `json:"icon"`
		TradeID string `json:"tradeId"`
	} `json:"items"`
}

// toCurrencyResponse converts a PoE2 response into the shared PoE1 format
// so the rest of the codebase can use one type.
func (r *poe2CurrencyResponse) toCurrencyResponse() *poeCurrencyResponse {
	out := &poeCurrencyResponse{}
	for _, l := range r.Lines {
		out.Lines = append(out.Lines, poeCurrencyLine{
			CurrencyTypeName: l.CurrencyTypeName,
			ChaosEquivalent:  l.PrimaryValue,
			ReceiveSparkLine: l.SparkLine,
			ListingCount:     l.ListingCount,
			DetailsID:        l.DetailsID,
		})
	}
	return out
}

// poeFetchCurrency fetches live currency data for a league and type (Currency or Fragment).
func poeFetchCurrency(league, currType string) (*poeCurrencyResponse, error) {
	return poeFetchCurrencyGame(league, currType, "poe1")
}

// poeFetchCurrencyGame fetches currency data for either PoE1 or PoE2.
// PoE2 uses a different endpoint path, different query param names, and a
// different response shape (primaryValue vs chaosEquivalent).
func poeFetchCurrencyGame(league, currType, game string) (*poeCurrencyResponse, error) {
	if game == "poe2" {
		// PoE2 uses a different base path and different param names.
		// URL: https://poe.ninja/api/data/poe2/currencyoverview
		//   OR https://poe.ninja/poe2/api/economy/currencyexchange/overview
		// Params: leagueName + overviewName (not league + type)
		candidates := []struct{ u, leagueKey, typeKey string }{
			{"https://poe.ninja/api/data/poe2/currencyoverview", "leagueName", "overviewName"},
			{"https://poe.ninja/poe2/api/economy/currencyexchange/overview", "leagueName", "overviewName"},
			{"https://poe.ninja/api/data/poe2/currencyoverview", "league", "type"},
		}
		var lastErr error
		for _, c := range candidates {
			params := url.Values{}
			params.Set(c.leagueKey, league)
			params.Set(c.typeKey, currType)
			resp, err := poeDoGet(c.u + "?" + params.Encode())
			if err != nil {
				lastErr = err
				continue
			}
			if resp.StatusCode != 200 {
				resp.Body.Close()
				lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, c.u)
				continue
			}
			var result poe2CurrencyResponse
			decErr := json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()
			if decErr != nil {
				lastErr = decErr
				continue
			}
			if len(result.Lines) > 0 {
				return result.toCurrencyResponse(), nil
			}
			// Empty lines — try next candidate
			lastErr = fmt.Errorf("empty response from %s", c.u)
		}
		return nil, fmt.Errorf("PoE2 currency API unavailable for league %q — check poe.ninja/poe2 for the current league name: %w", league, lastErr)
	}

	// PoE1 endpoint
	u := "https://poe.ninja/api/data/currencyoverview"
	params := url.Values{}
	params.Set("league", league)
	params.Set("type", currType)
	resp, err := poeDoGet(u + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("poe.ninja API returned HTTP %d for %s/%s — check league name (e.g. Mirage, Standard)", resp.StatusCode, league, currType)
	}
	var result poeCurrencyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing poe.ninja response: %w", err)
	}
	return &result, nil
}

// poeFetchItemGame fetches item data for a specific game version.
func poeFetchItemGame(league, itemType, game string) (*poeItemResponse, error) {
	var u string
	var params url.Values
	if game == "poe2" {
		u = "https://poe.ninja/api/data/poe2/itemoverview"
		params = url.Values{}
		params.Set("leagueName", league)
		params.Set("type", itemType) // PoE2 items still use "type"
	} else {
		u = "https://poe.ninja/api/data/itemoverview"
		params = url.Values{}
		params.Set("league", league)
		params.Set("type", itemType)
	}

	resp, err := poeDoGet(u + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("poe.ninja API returned HTTP %d for %s item %s/%s", resp.StatusCode, game, league, itemType)
	}
	var result poeItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing poe.ninja response: %w", err)
	}
	return &result, nil
}

// poeDefaultLeague returns a sensible default league name per game.
func poeDefaultLeague(game string) string {
	if game == "poe2" {
		return "Standard"
	}
	return "Mirage"
}

// poeFetchItem fetches live item data for a league and item type.
func poeFetchItem(league, itemType string) (*poeItemResponse, error) {
	u := "https://poe.ninja/api/data/itemoverview"
	params := url.Values{}
	params.Set("league", league)
	params.Set("type", itemType)

	resp, err := poeDoGet(u + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("poe.ninja API returned HTTP %d for item %s/%s — check that the league name is correct (run 'leagues' to see valid names)", resp.StatusCode, league, itemType)
	}
	var result poeItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing poe.ninja response: %w", err)
	}
	return &result, nil
}

var poeHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

func poeDoGet(rawURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "github.com/chewie2000/poe-ninja-pp-cli/1.0 (github.com/mvanhorn/printing-press-library)")
	req.Header.Set("Accept", "application/json")
	return poeHTTPClient.Do(req)
}

// poeLeagueNames are well-known default leagues.
var poeLeagueNames = []string{"Mirage", "Hardcore Mirage", "SSF Mirage", "HC SSF Mirage", "Standard", "Hardcore"}

// sparklineASCII renders a sparkline data array as a 30-char ASCII chart using block chars.
// Returns a string like "▁▂▃▄▅▆▇█" representing the trend.
func sparklineASCII(data []float64) string {
	if len(data) == 0 {
		return "no data"
	}
	// Filter out zero/nil points
	var valid []float64
	for _, v := range data {
		if v != 0 {
			valid = append(valid, v)
		}
	}
	if len(valid) == 0 {
		return "no data"
	}

	blocks := []rune("▁▂▃▄▅▆▇█")
	minV, maxV := valid[0], valid[0]
	for _, v := range valid {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}

	var sb strings.Builder
	for _, v := range valid {
		idx := 0
		if maxV > minV {
			idx = int(math.Round(float64(len(blocks)-1) * (v - minV) / (maxV - minV)))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		sb.WriteRune(blocks[idx])
	}
	return sb.String()
}

// sparklineVelocity computes (recent_slope - prior_slope) from a 7-point sparkline.
// Returns (velocity_per_day, acceleration_per_day).
func sparklineVelocity(data []float64) (float64, float64) {
	// Filter zeros
	var pts []float64
	for _, v := range data {
		if v != 0 {
			pts = append(pts, v)
		}
	}
	if len(pts) < 2 {
		return 0, 0
	}

	// Velocity = overall slope: (last - first) / (n-1)
	n := float64(len(pts))
	vel := (pts[len(pts)-1] - pts[0]) / (n - 1)

	if len(pts) < 4 {
		return vel, 0
	}

	// Split into halves for acceleration
	half := len(pts) / 2
	priorSlope := (pts[half-1] - pts[0]) / float64(half-1)
	recentSlope := (pts[len(pts)-1] - pts[half]) / float64(len(pts)-1-half)
	accel := recentSlope - priorSlope

	return vel, accel
}

// poeAllItemTypes returns all valid item types for itemoverview.
var poeAllItemTypes = []string{
	"Oil", "Incubator", "Scarab", "Fossil", "Resonator", "Essence",
	"DivinationCard", "SkillGem", "BaseType", "HelmetEnchant",
	"UniqueMap", "Map", "UniqueJewel", "UniqueFlask", "UniqueWeapon",
	"UniqueArmour", "UniqueAccessory", "Beast", "Vial", "DeliriumOrb",
	"Omen", "UniqueRelic", "ClusterJewel", "BlightedMap", "BlightRavagedMap",
	"Invitation", "Memory", "Coffin", "AllflameEmber",
}

// readAll reads all bytes from r.
func readAll(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}
