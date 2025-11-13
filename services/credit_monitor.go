package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// CreditInfo represents OpenRouter credit information
type CreditInfo struct {
	Data struct {
		Label          string   `json:"label"`
		Limit          *float64 `json:"limit"`
		LimitRemaining *float64 `json:"limit_remaining"`
		Usage          float64  `json:"usage"`
		UsageDaily     float64  `json:"usage_daily"`
		UsageWeekly    float64  `json:"usage_weekly"`
		UsageMonthly   float64  `json:"usage_monthly"`
		IsFreeTier     bool     `json:"is_free_tier"`
	} `json:"data"`
}

// CheckCredits queries OpenRouter API for current credit information
func CheckCredits() (*CreditInfo, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
	}

	req, _ := http.NewRequest("GET", "https://openrouter.ai/api/v1/auth/key", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var info CreditInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

// MonitorCredits runs continuous credit monitoring in background
func MonitorCredits() {
	// Initial check
	info, err := CheckCredits()
	if err != nil {
		log.Printf("⚠️  [CreditMonitor] Failed initial check: %v", err)
	} else {
		logCreditInfo(info)
	}

	// Check every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		info, err := CheckCredits()
		if err != nil {
			log.Printf("⚠️  [CreditMonitor] Error: %v", err)
			continue
		}

		logCreditInfo(info)

		// Alert if low credits
		if info.Data.LimitRemaining != nil {
			remaining := *info.Data.LimitRemaining
			if remaining < 1.0 {
				log.Printf("🔴 [CreditMonitor] CRITICAL: Low credits! Remaining: $%.2f", remaining)
				// TODO: Send alert (email, Slack, webhook, etc.)
			} else if remaining < 5.0 {
				log.Printf("🟡 [CreditMonitor] WARNING: Credits running low. Remaining: $%.2f", remaining)
			}
		}

		// Alert if high daily usage
		if info.Data.UsageDaily > 1.0 {
			log.Printf("🟡 [CreditMonitor] High daily usage: $%.2f", info.Data.UsageDaily)
		}
	}
}

// logCreditInfo logs credit information in a formatted way
func logCreditInfo(info *CreditInfo) {
	if info.Data.LimitRemaining != nil {
		log.Printf("💰 [CreditMonitor] Remaining: $%.2f | Daily: $%.4f | Weekly: $%.4f | Monthly: $%.4f",
			*info.Data.LimitRemaining,
			info.Data.UsageDaily,
			info.Data.UsageWeekly,
			info.Data.UsageMonthly)
	} else {
		// log.Printf("💰 [CreditMonitor] Daily: $%.4f | Weekly: $%.4f | Monthly: $%.4f | Total: $%.4f",
		// 	info.Data.UsageDaily,
		// 	info.Data.UsageWeekly,
		// 	info.Data.UsageMonthly,
		// 	info.Data.Usage)
	}
}
