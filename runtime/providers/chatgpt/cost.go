package chatgpt

import "strings"

// CalculateCost returns total USD cents (rounded half-up) at Standard pricing.
// Reasoning tokens must be included in outputTokens by the caller.
func (gpt *ChatGPT) CalculateCost(model string, inputTokens, outputTokens, cachedTokens int64) int64 {
	key := normalize(model)
	r, ok := std[key]
	if !ok {
		// Unknown model: charge zero. Log upstream if you want visibility.
		return 0
	}

	cachedRate := r.cachedMicroDollars
	if cachedRate == 0 {
		// Cached pricing unavailable => bill cached tokens as normal input.
		cachedRate = r.inMicroDollars
	}

	// We store prices as micro-dollars per 1M tokens.
	// Convert directly to cents with one half-up rounding:
	// cents = (tokens * (micro$/1M)) / (1_000_000 * 10_000 micro$/cent)
	const denom int64 = 10_000_000_000 // 1_000_000 * 10_000
	num := inputTokens*r.inMicroDollars +
		cachedTokens*cachedRate +
		outputTokens*r.outMicroDollars

	return halfUpDiv(num, denom)
}

type perMillion struct {
	inMicroDollars     int64 // input price per 1M tokens, in micro-dollars (1e-6 USD)
	cachedMicroDollars int64 // cached-input price per 1M tokens, in micro-dollars (0 => bill as input)
	outMicroDollars    int64 // output price per 1M tokens, in micro-dollars (1e-6 USD)
}

var std = map[string]perMillion{
	// GPT-5.x
	"gpt-5.2":             {1_750_000, 175_000, 14_000_000},
	"gpt-5.1":             {1_250_000, 125_000, 10_000_000},
	"gpt-5":               {1_250_000, 125_000, 10_000_000},
	"gpt-5-mini":          {250_000, 25_000, 2_000_000},
	"gpt-5-nano":          {50_000, 5_000, 400_000},
	"gpt-5.2-chat-latest": {1_750_000, 175_000, 14_000_000},
	"gpt-5.1-chat-latest": {1_250_000, 125_000, 10_000_000},
	"gpt-5-chat-latest":   {1_250_000, 125_000, 10_000_000},

	// Codex
	"gpt-5.1-codex-max":  {1_250_000, 125_000, 10_000_000},
	"gpt-5.1-codex":      {1_250_000, 125_000, 10_000_000},
	"gpt-5-codex":        {1_250_000, 125_000, 10_000_000},
	"gpt-5.1-codex-mini": {250_000, 25_000, 2_000_000},
	"codex-mini-latest":  {1_500_000, 375_000, 6_000_000},

	// Pro
	"gpt-5.2-pro": {21_000_000, 0, 168_000_000},
	"gpt-5-pro":   {15_000_000, 0, 120_000_000},

	// GPT-4.1 / 4o
	"gpt-4.1":           {2_000_000, 500_000, 8_000_000},
	"gpt-4.1-mini":      {400_000, 100_000, 1_600_000},
	"gpt-4.1-nano":      {100_000, 25_000, 400_000},
	"gpt-4o":            {2_500_000, 1_250_000, 10_000_000},
	"gpt-4o-2024-05-13": {5_000_000, 0, 15_000_000},
	"gpt-4o-mini":       {150_000, 75_000, 600_000},

	// Realtime (text tokens)
	"gpt-realtime":                 {4_000_000, 400_000, 16_000_000},
	"gpt-realtime-mini":            {600_000, 60_000, 2_400_000},
	"gpt-4o-realtime-preview":      {5_000_000, 2_500_000, 20_000_000},
	"gpt-4o-mini-realtime-preview": {600_000, 300_000, 2_400_000},

	// Audio (text tokens)
	"gpt-audio":                 {2_500_000, 0, 10_000_000},
	"gpt-audio-mini":            {600_000, 0, 2_400_000},
	"gpt-4o-audio-preview":      {2_500_000, 0, 10_000_000},
	"gpt-4o-mini-audio-preview": {150_000, 0, 600_000},

	// o-series
	"o1":                    {15_000_000, 7_500_000, 60_000_000},
	"o1-pro":                {150_000_000, 0, 600_000_000},
	"o1-mini":               {1_100_000, 550_000, 4_400_000},
	"o3":                    {2_000_000, 500_000, 8_000_000},
	"o3-pro":                {20_000_000, 0, 80_000_000},
	"o3-mini":               {1_100_000, 550_000, 4_400_000},
	"o3-deep-research":      {10_000_000, 2_500_000, 40_000_000},
	"o4-mini":               {1_100_000, 275_000, 4_400_000},
	"o4-mini-deep-research": {2_000_000, 500_000, 8_000_000},

	// Search / tools
	"gpt-5-search-api":           {1_250_000, 125_000, 10_000_000},
	"gpt-4o-mini-search-preview": {150_000, 0, 600_000},
	"gpt-4o-search-preview":      {2_500_000, 0, 10_000_000},
	"computer-use-preview":       {3_000_000, 0, 12_000_000},

	// Image (text tokens)
	"gpt-image-1.5":        {5_000_000, 1_250_000, 10_000_000},
	"chatgpt-image-latest": {5_000_000, 1_250_000, 10_000_000},
	"gpt-image-1":          {5_000_000, 1_250_000, 0},
	"gpt-image-1-mini":     {2_000_000, 200_000, 0},
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
