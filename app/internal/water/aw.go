package water

import (
	"context"
	"encoding/json"
	"net/url"
)

// GaugeParams is the American Whitewater gauge readings request.
type GaugeParams struct {
	DesiredMetric         string `json:"desiredMetric,omitempty"`
	TimePeriod            string `json:"timePeriod,omitempty"`
	GaugeSource           string `json:"gaugeSource,omitempty"`
	GaugeSourceIdentifier string `json:"gaugeSourceIdentifier"`
}

// GaugeReadings returns American Whitewater gauge readings
// ([{value, dateTime, qualifier}, ...]) exactly as the API sends them, or an
// empty array when the response has an unexpected shape.
func (c *Client) GaugeReadings(ctx context.Context, p GaugeParams) (json.RawMessage, error) {
	if p.DesiredMetric == "" {
		p.DesiredMetric = "cfs"
	}
	if p.TimePeriod == "" {
		p.TimePeriod = "week"
	}
	if p.GaugeSource == "" {
		p.GaugeSource = "CALCULATED"
	}
	input, err := json.Marshal(map[string]any{"0": map[string]any{"json": p}})
	if err != nil {
		return nil, err
	}
	q := url.Values{"batch": {"1"}, "input": {string(input)}}
	_, body, err := c.get(ctx, c.AWURL+"?"+q.Encode())
	if err != nil {
		return nil, err
	}

	var batch []struct {
		Result struct {
			Data struct {
				JSON json.RawMessage `json:"json"`
			} `json:"data"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &batch) != nil || len(batch) == 0 || !isArray(batch[0].Result.Data.JSON) {
		return json.RawMessage("[]"), nil
	}
	return batch[0].Result.Data.JSON, nil
}

func isArray(b json.RawMessage) bool {
	var v []json.RawMessage
	return json.Unmarshal(b, &v) == nil
}
