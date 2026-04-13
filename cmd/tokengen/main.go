package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ArmanAA/rain-staking/internal/auth"
)

func main() {
	customerID := flag.String("customer-id", "550e8400-e29b-41d4-a716-446655440000", "Customer ID for the token")
	secret := flag.String("secret", "dev-secret-do-not-use-in-production", "JWT signing secret")
	expiry := flag.Duration("expiry", 24*time.Hour, "Token expiry duration")
	flag.Parse()

	token, err := auth.GenerateToken(*customerID, *secret, *expiry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(token)
}
