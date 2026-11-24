package water

import (
	"context"
	"net/url"
)

// CDEC returns the raw JSON from the CDEC JSONDataServlet for a station's
// sensor 1 (event duration) since start, plus its Content-Type.
func (c *Client) CDEC(ctx context.Context, station, start string) ([]byte, string, error) {
	q := url.Values{"Stations": {station}, "SensorNums": {"1"}, "dur_code": {"E"}, "Start": {start}}
	resp, body, err := c.get(ctx, c.CDECURL+"?"+q.Encode())
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}
