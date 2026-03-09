package webhooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"infinite-experiment/politburo/internal/flights"
)

// WebhookTypeLiveFlights is the webhook_type value for live flights Discord notifications
const WebhookTypeLiveFlights = "live_flights"

// Discord webhook payload structures (subset of what Discord accepts)
// See https://discord.com/developers/docs/resources/webhook#execute-webhook

type discordWebhookPayload struct {
	Content string         `json:"content,omitempty"`
	Embeds  []discordEmbed `json:"embeds,omitempty"`
}

type discordEmbed struct {
	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	Color       int           `json:"color,omitempty"` // decimal (e.g. 0x5865F2 = 5793266)
	Fields      []discordField `json:"fields,omitempty"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

const discordFieldValueMaxLen = 1024

// formatLiveFlightBlock returns the compact per-flight block (Callsign, IFC Username, Equipment, Telemetry, Route)
func formatLiveFlightBlock(f *flights.VALiveFlightDTO) string {
	equipment := strings.TrimSpace(f.LiveryName + " " + f.AircraftName)
	if equipment == "" {
		equipment = "—"
	}
	route := f.Origin + "-" + f.Destination
	if f.Origin == "" && f.Destination == "" {
		route = "—"
	}
	username := f.Username
	if username == "" {
		username = "—"
	}
	return fmt.Sprintf("Callsign: %s\nIFC Username: %s\nEquipment: %s\nTelemetry: %dft @ %dkts\nRoute: %s",
		f.Callsign, username, equipment, f.Altitude, f.Speed, route)
}

// BuildLiveFlightsPayload builds a Discord webhook JSON body for the live flights snapshot.
// vaName is used in the embed title; snapshotTime for the description; flights can be empty.
func BuildLiveFlightsPayload(vaName string, flts []flights.VALiveFlightDTO, snapshotTime time.Time) ([]byte, error) {
	title := "Live Flights — " + vaName
	if vaName == "" {
		title = "Live Flights"
	}
	desc := fmt.Sprintf("Snapshot at %s. %d flight(s) live.", snapshotTime.UTC().Format(time.RFC3339), len(flts))
	if len(flts) == 0 {
		desc = fmt.Sprintf("No live flights at %s.", snapshotTime.UTC().Format(time.RFC3339))
	}

	embed := discordEmbed{
		Title:       title,
		Description: desc,
		Color:       5793266, // Discord blurple
		Fields:      nil,
	}

	if len(flts) > 0 {
		// One field per flight (name = callsign, value = compact block) to stay under 1024 chars per value
		embed.Fields = make([]discordField, 0, len(flts))
		for i := range flts {
			block := formatLiveFlightBlock(&flts[i])
			if len(block) > discordFieldValueMaxLen {
				block = block[:discordFieldValueMaxLen-3] + "..."
			}
			embed.Fields = append(embed.Fields, discordField{
				Name:   flts[i].Callsign,
				Value:  block,
				Inline: false,
			})
		}
	}

	payload := discordWebhookPayload{
		Embeds: []discordEmbed{embed},
	}
	return json.Marshal(payload)
}
