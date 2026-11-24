// Package water fetches river and gauge data from fixed public sources
// (CDEC, American Whitewater, EID, NOAA CNRFC). Hosts are fixed; user input
// only ever goes into URL-encoded query or form values.
package water

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxResponseBytes = 10 << 20

// Client calls the upstream sources. The URL fields default to the real
// services and are overridable for tests.
type Client struct {
	HTTP    *http.Client
	CDECURL string
	AWURL   string
	EIDURL  string
	NOAAURL string
}

func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
			// Like the original EID scraper: don't follow redirects.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		CDECURL: "https://cdec.water.ca.gov/dynamicapp/req/JSONDataServlet",
		AWURL:   "https://trpc-api.americanwhitewater.org/gauge/gaugeReadingsHistory",
		EIDURL:  "https://project184.eid.org/Default.aspx",
		NOAAURL: "https://www.cnrfc.noaa.gov/graphicalRVF-inflows_csv.php",
	}
}

// do sends req and returns the body of a 2xx or 3xx response.
func (c *Client) do(req *http.Request) (*http.Response, []byte, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("%s: status %d", req.URL.Host, resp.StatusCode)
	}
	return resp, body, nil
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	return c.do(req)
}
