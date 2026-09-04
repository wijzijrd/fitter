// webhooksetup registers (or inspects) fitter's single Strava webhook
// subscription. Strava allows exactly one subscription per API application,
// so this is a one-time operation against the deployed public backend, run
// by hand — not something the server does automatically on every boot.
//
// Usage:
//
//	go run ./cmd/webhooksetup create   # register PUBLIC_BASE_URL/webhooks/strava
//	go run ./cmd/webhooksetup list     # show the current subscription, if any
//	go run ./cmd/webhooksetup delete <id>
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/fitter/svc/internal/strava"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: webhooksetup <create|list|delete> [id]")
		os.Exit(1)
	}

	client := strava.NewClient(strava.Config{
		ClientID:           mustEnv("STRAVA_CLIENT_ID"),
		ClientSecret:       mustEnv("STRAVA_CLIENT_SECRET"),
		WebhookVerifyToken: mustEnv("STRAVA_WEBHOOK_VERIFY_TOKEN"),
	})
	ctx := context.Background()

	switch os.Args[1] {
	case "create":
		callbackURL := mustEnv("PUBLIC_BASE_URL") + "/webhooks/strava"
		sub, err := client.CreateSubscription(ctx, callbackURL)
		fatalIf(err)
		fmt.Printf("created subscription id=%d callback_url=%s\n", sub.ID, sub.CallbackURL)
	case "list":
		subs, err := client.ListSubscriptions(ctx)
		fatalIf(err)
		if len(subs) == 0 {
			fmt.Println("no subscriptions")
			return
		}
		for _, s := range subs {
			fmt.Printf("id=%d callback_url=%s\n", s.ID, s.CallbackURL)
		}
	case "delete":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: webhooksetup delete <id>")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		fatalIf(err)
		fatalIf(client.DeleteSubscription(ctx, id))
		fmt.Println("deleted")
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		os.Exit(1)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "missing required env var %s\n", key)
		os.Exit(1)
	}
	return v
}

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
