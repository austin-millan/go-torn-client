package client

import "encoding/json"

// UserPathID builds a /user/{id}/... path parameter from a Torn user id.
func UserPathID(id int32) UserDiscordPathId {
	raw, err := json.Marshal(id)
	if err != nil {
		// int32 always marshals
		raw = []byte("0")
	}
	return UserDiscordPathId{union: raw}
}

// DiscordPathID builds a /user/{id}/... path parameter from a Discord snowflake.
func DiscordPathID(discordID string) UserDiscordPathId {
	raw, err := json.Marshal(discordID)
	if err != nil {
		raw = []byte(`""`)
	}
	return UserDiscordPathId{union: raw}
}
