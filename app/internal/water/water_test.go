package water

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func fake(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient()
	c.CDECURL, c.AWURL, c.EIDURL, c.NOAAURL = srv.URL+"/cdec", srv.URL+"/aw", srv.URL+"/eid", srv.URL+"/noaa"
	return c
}

func TestCDECEncodesInput(t *testing.T) {
	var got map[string][]string
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"value":1}]`))
	})
	body, ct, err := c.CDEC(context.Background(), "FOL&Stations=EVIL", "2026-05-01")
	if err != nil || string(body) != `[{"value":1}]` || ct != "application/json" {
		t.Fatalf("got %s %q %v", body, ct, err)
	}
	want := map[string][]string{"Stations": {"FOL&Stations=EVIL"}, "SensorNums": {"1"}, "dur_code": {"E"}, "Start": {"2026-05-01"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("query = %v, want %v", got, want)
	}
}

func TestGaugeReadings(t *testing.T) {
	var input map[string]map[string]GaugeParams
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("batch") != "1" {
			t.Errorf("batch = %q", r.URL.Query().Get("batch"))
		}
		json.Unmarshal([]byte(r.URL.Query().Get("input")), &input)
		w.Write([]byte(`[{"result":{"data":{"json":[{"value":"1400","dateTime":"2026-05-07T18:00:00.000Z","qualifier":"NORMAL"}]}}}]`))
	})
	got, err := c.GaugeReadings(context.Background(), GaugeParams{GaugeSourceIdentifier: "35795422", TimePeriod: "day"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `[{"value":"1400","dateTime":"2026-05-07T18:00:00.000Z","qualifier":"NORMAL"}]` {
		t.Errorf("readings = %s", got)
	}
	want := GaugeParams{DesiredMetric: "cfs", TimePeriod: "day", GaugeSource: "CALCULATED", GaugeSourceIdentifier: "35795422"}
	if input["0"]["json"] != want {
		t.Errorf("input = %+v, want %+v", input, want)
	}
}

func TestGaugeReadingsUnexpectedShape(t *testing.T) {
	for _, body := range []string{`{}`, `[]`, `[{"result":{}}]`, `[{"result":{"data":{"json":{"x":1}}}}]`, `not json`} {
		c := fake(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) })
		got, err := c.GaugeReadings(context.Background(), GaugeParams{GaugeSourceIdentifier: "1"})
		if err != nil || string(got) != "[]" {
			t.Errorf("%s: got %s %v, want []", body, got, err)
		}
	}
}

func TestNOAA(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "FOCC1" {
			t.Errorf("id = %q", r.URL.Query().Get("id"))
		}
		// Header, PDT (UTC-7), PST (UTC-8), PM hour, number with junk, blank flow, bad date.
		fmt.Fprint(w, "Date,Flow,Trend\n05/06/2026 08 AM,395,\n01/15/2026 08 AM,380,-\n\n05/06/2026 01 PM,12.5cfs,+\n05/06/2026 02 PM,,\nbad,7,\n")
	})
	got, err := c.NOAA(context.Background(), "FOCC1")
	if err != nil {
		t.Fatal(err)
	}
	want := []NOAAPoint{
		{"2026-05-06T15:00:00.000Z", 395, ""},
		{"2026-01-15T16:00:00.000Z", 380, "-"},
		{"2026-05-06T20:00:00.000Z", 12.5, "+"},
		{"2026-05-06T21:00:00.000Z", 0, ""},
		{"", 7, ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestUpstreamErrorStatus(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", 503) })
	if _, err := c.NOAA(context.Background(), "X"); err == nil {
		t.Error("want error for 503")
	}
}

func eidHTML(state string, rows ...[3]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<input type="hidden" id="__VIEWSTATE" value="vs-%s" /><input id="__VIEWSTATEGENERATOR" value="gen" /><input id="__EVENTVALIDATION" value="ev-%s" /><table>`, state, state)
	for _, r := range rows {
		fmt.Fprintf(&b, "<tr class=\"row\">\n<td>%s</td>\n<td>%s</td>\n<td>%s</td>\n</tr>", r[0], r[1], r[2])
	}
	return b.String() + "</table>"
}

func TestEIDPaginates(t *testing.T) {
	var posts []map[string][]string
	var cookies []string
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query(); q.Get("id") != "12" || q.Get("projectid") != "3" {
			t.Errorf("query = %v", q)
		}
		if r.Method == http.MethodGet {
			http.SetCookie(w, &http.Cookie{Name: "ASP.NET_SessionId", Value: "abc", Path: "/", HttpOnly: true})
			fmt.Fprint(w, eidHTML("1", [3]string{"5/6/2026 8:00 AM", "3.25 ft.", "1,234 cfs"}, [3]string{"5/6/2026 7:45 AM", "3.20 ft.", "1,200 cfs"}))
			return
		}
		body, _ := io.ReadAll(r.Body)
		form, _ := parseForm(string(body))
		posts = append(posts, form)
		cookies = append(cookies, r.Header.Get("Cookie"))
		switch form["__EVENTARGUMENT"][0] {
		case "Page$2":
			fmt.Fprint(w, eidHTML("2", [3]string{"5/6/2026 7:30 AM", "3.10 ft.", "999.5 cfs"}))
		default: // page 3 repeats page 1: stop
			fmt.Fprint(w, eidHTML("3", [3]string{"5/6/2026 8:00 AM", "3.25 ft.", "1,234 cfs"}))
		}
	})
	got, err := c.EID(context.Background(), slog.New(slog.DiscardHandler), "12", "3")
	if err != nil {
		t.Fatal(err)
	}
	want := []EIDPoint{
		{"5/6/2026 8:00 AM", "3.25 ft.", 3.25, "ft", "1,234 cfs", 1234, "cfs"},
		{"5/6/2026 7:45 AM", "3.20 ft.", 3.2, "ft", "1,200 cfs", 1200, "cfs"},
		{"5/6/2026 7:30 AM", "3.10 ft.", 3.1, "ft", "999.5 cfs", 999.5, "cfs"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	if len(posts) != 2 {
		t.Fatalf("posts = %d, want 2 (page 2, then page 3 duplicate)", len(posts))
	}
	// Each page posts the view state from the previous page.
	if posts[0]["__VIEWSTATE"][0] != "vs-1" || posts[1]["__VIEWSTATE"][0] != "vs-2" || posts[1]["__EVENTVALIDATION"][0] != "ev-2" {
		t.Errorf("view state not carried: %v", posts)
	}
	if posts[0]["__EVENTTARGET"][0] != "ctl00$MainContent$gvCurrentWaterData" {
		t.Errorf("event target = %v", posts[0]["__EVENTTARGET"])
	}
	if cookies[0] != "ASP.NET_SessionId=abc" {
		t.Errorf("cookie = %q", cookies[0])
	}
}

func TestEIDStopsWithoutViewState(t *testing.T) {
	posts := 0
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
		}
		fmt.Fprint(w, "<table><tr><td>5/6/2026 8:00 AM</td><td>3.25 ft.</td><td>10 cfs</td></tr></table>")
	})
	got, err := c.EID(context.Background(), slog.New(slog.DiscardHandler), "10", "1")
	if err != nil || len(got) != 1 || posts != 0 {
		t.Errorf("got %d rows, %d posts, err %v", len(got), posts, err)
	}
}

func parseForm(s string) (map[string][]string, error) {
	req, _ := http.NewRequest("POST", "/", strings.NewReader(s))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	err := req.ParseForm()
	return req.PostForm, err
}
