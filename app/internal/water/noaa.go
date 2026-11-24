package water

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NOAAPoint is one forecast row from the CNRFC inflow CSV.
type NOAAPoint struct {
	DateTime string  `json:"dateTime"` // UTC, e.g. 2026-05-06T15:00:00.000Z
	Flow     float64 `json:"flow"`
	Trend    string  `json:"trend"`
}

var pacific = mustLoad("America/Los_Angeles")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// NOAA returns the CNRFC inflow forecast for a station, with times
// converted from Pacific time to UTC.
func (c *Client) NOAA(ctx context.Context, stationID string) ([]NOAAPoint, error) {
	_, body, err := c.get(ctx, c.NOAAURL+"?"+url.Values{"id": {stationID}}.Encode())
	if err != nil {
		return nil, err
	}
	return parseNOAA(string(body)), nil
}

// parseNOAA parses rows like "05/06/2026 08 AM,395," after a header row.
func parseNOAA(csv string) []NOAAPoint {
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	points := []NOAAPoint{}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		p := NOAAPoint{DateTime: pacificToUTC(parts[0])}
		if len(parts) > 1 {
			p.Flow = leadingFloat(parts[1])
		}
		if len(parts) > 2 {
			p.Trend = parts[2]
		}
		points = append(points, p)
	}
	return points
}

func pacificToUTC(s string) string {
	t, err := time.ParseInLocation("01/02/2006 03 PM", s, pacific)
	if err != nil {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

var leadingNumber = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)`)

// leadingFloat parses the number at the start of s, or 0 (like JavaScript's
// parseFloat(s) || 0).
func leadingFloat(s string) float64 {
	f, _ := strconv.ParseFloat(leadingNumber.FindString(s), 64)
	return f
}
