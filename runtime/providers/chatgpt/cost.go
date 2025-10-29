package chatgpt

import "strings"

// CalculateCost returns total USD cents (rounded half-up) at Standard pricing.
// Reasoning tokens must be included in outputTokens by the caller.
func (chat *ChatGPT) CalculateCost(model string, inputTokens, outputTokens, cachedTokens int64) int64 {
	key := normalize(model)
	r, ok := std[key]
	if !ok {
		// Unknown model: charge zero. Log upstream if you want visibility.
		return 0
	}

	cached := r.cachedCents
	if cached == 0 {
		cached = r.inCents
	}

	// cost = tokens * (cents per 1M) / 1_000_000, with half-up rounding
	const perMillion int64 = 1_000_000

	inCost := halfUpDiv(inputTokens*r.inCents, perMillion)
	caCost := halfUpDiv(cachedTokens*cached, perMillion)
	outCost := halfUpDiv(outputTokens*r.outCents, perMillion)

	return inCost + caCost + outCost
}

type perMillion struct {
	inCents     int64 // input price per 1M tokens, in USD cents
	cachedCents int64 // cached-input price per 1M tokens, in USD cents (0 => bill as input)
	outCents    int64 // output price per 1M tokens, in USD cents
}

var std = map[string]perMillion{
	"gpt-5":                        {125, 12, 1000},
	"gpt-5-mini":                   {25, 2, 200},
	"gpt-5-nano":                   {5, 0, 40}, // cached 0? Standard table says 0.005 => 0 (round to nearest cent per 1M) – handle via input fallback if you prefer
	"gpt-5-chat-latest":            {125, 12, 1000},
	"gpt-5-codex":                  {125, 12, 1000},
	"gpt-5-pro":                    {1500, 0, 12000},
	"gpt-4.1":                      {200, 50, 800},
	"gpt-4.1-mini":                 {40, 10, 160},
	"gpt-4.1-nano":                 {10, 2, 40},
	"gpt-4o":                       {250, 125, 1000},
	"gpt-4o-2024-05-13":            {500, 0, 1500},
	"gpt-4o-mini":                  {15, 7, 60},
	"gpt-realtime":                 {400, 40, 1600},
	"gpt-realtime-mini":            {60, 6, 240},
	"gpt-4o-realtime-preview":      {500, 250, 2000},
	"gpt-4o-mini-realtime-preview": {60, 30, 240},
	"gpt-audio":                    {250, 0, 1000},
	"gpt-audio-mini":               {60, 0, 240},
	"gpt-4o-audio-preview":         {250, 0, 1000},
	"gpt-4o-mini-audio-preview":    {15, 0, 60},
	"o1":                           {1500, 750, 6000},
	"o1-pro":                       {15000, 0, 60000},
	"o3-pro":                       {2000, 0, 8000},
	"o3":                           {200, 50, 800},
	"o3-deep-research":             {1000, 250, 4000},
	"o4-mini":                      {110, 27, 440},
	"o4-mini-deep-research":        {200, 50, 800},
	"o3-mini":                      {110, 55, 440},
	"o1-mini":                      {110, 55, 440},
	"codex-mini-latest":            {150, 37, 600},
	"gpt-5-search-api":             {125, 12, 1000},
	"gpt-4o-mini-search-preview":   {15, 0, 60},
	"gpt-4o-search-preview":        {250, 0, 1000},
	"computer-use-preview":         {300, 0, 1200},
	"gpt-image-1":                  {500, 125, 0},
	"gpt-image-1-mini":             {200, 20, 0},
}

// normalize maps common aliases to Standard table keys.
// Extend as needed where your API accepts different strings.
func normalize(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	switch m {
	case "gpt-5-auto", "gpt-5-latest":
		return "gpt-5"
	case "gpt-4o-latest":
		return "gpt-4o"
	case "gpt-5-code", "gpt-5-coder":
		return "gpt-5-codex"
	default:
		return m
	}
}

// halfUpDiv does (a/b) with half-up rounding for non-negative integers.
// Assumes a,b >= 0 and b > 0.
func halfUpDiv(a, b int64) int64 {
	return (a + b/2) / b
}
