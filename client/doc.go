// Package client is a complete typed client for Torn API v2
// (https://api.torn.com/v2), generated from Torn's published OpenAPI spec.
//
// Typical usage:
//
//	api, err := client.New(os.Getenv("TORN_API_KEY"),
//		client.WithUserAgent("my-bot/1.0"),
//		client.WithComment("my-bot"),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//	resp, err := api.GetMyBarsWithResponse(context.Background(), nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(resp.JSON200.Bars.Energy.Current)
//
// Regenerate models and endpoint methods after fetching a fresh spec:
//
//	FETCH_SPEC=1 ./scripts/generate.sh
package client
