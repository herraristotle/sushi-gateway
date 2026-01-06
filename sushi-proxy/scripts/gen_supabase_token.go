package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	secret := "mAHNt5/rhlD6yMPzIxCNS5Yrw5+qtkGWhNDgLhkTxhcxjWGXWS24SnDYiqoDrg20d5KddjM8IOKTjXtnxcsnVQ=="
	issuer := "https://tsqoipgfjxscwnjqxyfj.supabase.co/auth/v1"

	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "test-user-123",
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour * 1).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		fmt.Printf("Error signing token: %v\n", err)
		return
	}

	fmt.Println(tokenString)
}
