// Example: fetch your bars and basic profile from Torn API v2.
//
//	export TORN_API_KEY=...
//	go run ./examples/basic
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gitlab.com/torn-city/go-torn-client/client"
)

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("TORN_API_KEY")
	if apiKey == "" {
		log.Fatal("set TORN_API_KEY in the environment or .env")
	}

	api, err := client.New(apiKey,
		client.WithUserAgent("gitlab.com/torn-city/go-torn-client"),
		client.WithComment("gitlab.com/torn-city/go-torn-client"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	basic, err := api.GetMyBasicInformationWithResponse(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	profile := basic.JSON200.Profile
	fmt.Printf("%s [%d] level %d\n", profile.Name, profile.Id, profile.Level)

	bars, err := api.GetMyBarsWithResponse(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	energy := bars.JSON200.Bars.Energy
	fmt.Printf("energy %d/%d\n", energy.Current, energy.Maximum)
}
