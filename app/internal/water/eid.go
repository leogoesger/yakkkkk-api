package water

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// EIDPoint is one row of the EID current water data table.
type EIDPoint struct {
	DateTime   string  `json:"dateTime"`
	Level      string  `json:"level"`
	LevelValue float64 `json:"levelValue"`
	LevelUnit  string  `json:"levelUnit"`
	Flow       string  `json:"flow"`
	FlowValue  float64 `json:"flowValue"`
	FlowUnit   string  `json:"flowUnit"`
}

const (
	eidMaxPages     = 5
	eidPageUA       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	eidPaginationUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:149.0) Gecko/20100101 Firefox/149.0"
)

// EID scrapes the EID water data page (https://project184.eid.org) for a
// site and project, following the ASP.NET grid pagination up to 5 pages.
func (c *Client) EID(ctx context.Context, log *slog.Logger, siteID, projectID string) ([]EIDPoint, error) {
	pageURL := c.EIDURL + "?" + url.Values{"id": {siteID}, "projectid": {projectID}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", eidPageUA)
	resp, body, err := c.do(req)
	if err != nil {
		return nil, err
	}
	html := string(body)
	all := parseEIDTable(html)
	cookie := cookieHeader(resp.Cookies())

	for page := 2; page <= eidMaxPages; page++ {
		form := viewState(html)
		if form == nil {
			break
		}
		html, err = c.eidPage(ctx, pageURL, page, form, cookie)
		if err != nil {
			log.Warn("eid page failed", "page", page, "err", err)
			break
		}
		rows := parseEIDTable(html)
		// Stop when there is no more data, or the site returned the first page again.
		if len(rows) == 0 || (len(all) > 0 && all[0].DateTime == rows[0].DateTime) {
			break
		}
		all = append(all, rows...)
	}
	return all, nil
}

func (c *Client) eidPage(ctx context.Context, pageURL string, page int, form url.Values, cookie string) (string, error) {
	form.Set("__EVENTTARGET", "ctl00$MainContent$gvCurrentWaterData")
	form.Set("__EVENTARGUMENT", "Page$"+strconv.Itoa(page))
	form.Set("__LASTFOCUS", "")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pageURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", eidPaginationUA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Referer", pageURL)
	req.Header.Set("Origin", "https://project184.eid.org")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	_, body, err := c.do(req)
	return string(body), err
}

var (
	eidRow   = regexp.MustCompile(`(?i)<tr[^>]*>[\s\S]*?<td[^>]*>([\d/\s:APM]+)</td>[\s\S]*?<td[^>]*>([\d.]+\s*ft\.)</td>[\s\S]*?<td[^>]*>([\d.,]+\s*cfs)</td>[\s\S]*?</tr>`)
	eidLevel = regexp.MustCompile(`([\d.]+)\s*(\w+)`)
	eidFlow  = regexp.MustCompile(`([\d.,]+)\s*(\w+)`)

	aspFields = []string{"__VIEWSTATE", "__VIEWSTATEGENERATOR", "__EVENTVALIDATION"}
)

func parseEIDTable(html string) []EIDPoint {
	points := []EIDPoint{}
	for _, m := range eidRow.FindAllStringSubmatch(html, -1) {
		level, flow := strings.TrimSpace(m[2]), strings.TrimSpace(m[3])
		lm, fm := eidLevel.FindStringSubmatch(level), eidFlow.FindStringSubmatch(flow)
		if lm == nil || fm == nil {
			continue
		}
		points = append(points, EIDPoint{
			DateTime:   strings.TrimSpace(m[1]),
			Level:      level,
			LevelValue: leadingFloat(lm[1]),
			LevelUnit:  lm[2],
			Flow:       flow,
			FlowValue:  leadingFloat(strings.ReplaceAll(fm[1], ",", "")),
			FlowUnit:   fm[2],
		})
	}
	return points
}

// viewState extracts the ASP.NET hidden form fields, or nil if any is missing.
func viewState(html string) url.Values {
	form := url.Values{}
	for _, f := range aspFields {
		m := regexp.MustCompile(`id="` + f + `"[^>]*value="([^"]+)"`).FindStringSubmatch(html)
		if m == nil {
			return nil
		}
		form.Set(f, m[1])
	}
	return form
}

func cookieHeader(cookies []*http.Cookie) string {
	parts := make([]string, len(cookies))
	for i, c := range cookies {
		parts[i] = c.Name + "=" + c.Value
	}
	return strings.Join(parts, "; ")
}
