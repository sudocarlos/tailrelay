package caddy

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// MetricsData holds parsed Caddy Prometheus metrics.
type MetricsData struct {
	Hosts     []HostMetrics    `json:"hosts"`
	Upstreams []UpstreamHealth `json:"upstreams"`
}

// HostMetrics holds per-host request and bandwidth counters.
// When per_host is enabled, Caddy adds a "host" label to HTTP middleware metrics.
// Without per_host the single entry will have Host == "".
type HostMetrics struct {
	Host         string             `json:"host"`
	Label        string             `json:"label"` // e.g. ":8888 → whoami-test:80"; empty when unknown
	Requests     float64            `json:"requests"`
	RequestsIn   float64            `json:"requests_in"`
	ResponsesOut float64            `json:"responses_out"`
	StatusCodes  map[string]float64 `json:"status_codes"` // "2xx","3xx","4xx","5xx"
}

// UpstreamHealth holds per-upstream health status from the reverse proxy.
type UpstreamHealth struct {
	Address string  `json:"address"`
	Healthy float64 `json:"healthy"`
}

// ParseMetrics parses Prometheus text-format metrics from Caddy and returns
// structured MetricsData. Only the metric families relevant to the dashboard
// are extracted; all others are skipped.
func ParseMetrics(raw []byte) *MetricsData {
	// Intermediate accumulators keyed by host label value.
	type hostAcc struct {
		requests     float64
		requestsIn   float64
		responsesOut float64
		statusCodes  map[string]float64
	}
	hosts := map[string]*hostAcc{}
	upstreams := map[string]float64{}

	getHost := func(h string) *hostAcc {
		if _, ok := hosts[h]; !ok {
			hosts[h] = &hostAcc{statusCodes: map[string]float64{}}
		}
		return hosts[h]
	}

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments and blank lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, labels, value, ok := parseLine(line)
		if !ok {
			continue
		}

		switch {
		// caddy_http_requests_total — request count per server/handler(/host)
		case name == "caddy_http_requests_total":
			// Only count the reverse_proxy or subroute handler to avoid double-counting
			// (each request passes through multiple handlers). Routes built with a
			// subroute wrapper emit metrics with handler="subroute" at the outer level.
			h := labels["handler"]
			if h != "reverse_proxy" && h != "subroute" {
				continue
			}
			host := labels["host"] // empty string when per_host disabled
			getHost(host).requests += value

		// caddy_http_request_size_bytes_sum — bytes received (request bodies)
		case name == "caddy_http_request_size_bytes_sum":
			h := labels["handler"]
			if h != "reverse_proxy" && h != "subroute" {
				continue
			}
			host := labels["host"]
			getHost(host).requestsIn += value

		// caddy_http_response_size_bytes_sum — bytes sent (response bodies)
		case name == "caddy_http_response_size_bytes_sum":
			h := labels["handler"]
			if h != "reverse_proxy" && h != "subroute" {
				continue
			}
			host := labels["host"]
			getHost(host).responsesOut += value

		// caddy_http_request_duration_seconds_count — has code + host labels,
		// use it to tally status-code class counts per host.
		case name == "caddy_http_request_duration_seconds_count":
			h := labels["handler"]
			if h != "reverse_proxy" && h != "subroute" {
				continue
			}
			host := labels["host"]
			code := labels["code"]
			class := statusClass(code)
			if class != "" {
				getHost(host).statusCodes[class] += value
			}

		// caddy_reverse_proxy_upstreams_healthy — upstream health gauge
		case name == "caddy_reverse_proxy_upstreams_healthy":
			upstream := labels["upstream"]
			if upstream != "" {
				upstreams[upstream] = value
			}
		}
	}

	// Build output slices.
	data := &MetricsData{}

	for host, acc := range hosts {
		data.Hosts = append(data.Hosts, HostMetrics{
			Host:         host,
			Requests:     acc.requests,
			RequestsIn:   acc.requestsIn,
			ResponsesOut: acc.responsesOut,
			StatusCodes:  acc.statusCodes,
		})
	}

	for addr, healthy := range upstreams {
		data.Upstreams = append(data.Upstreams, UpstreamHealth{
			Address: addr,
			Healthy: healthy,
		})
	}

	return data
}

// statusClass maps an HTTP status code string to its class label ("2xx"…"5xx").
// Returns "" for unrecognised or 1xx codes.
func statusClass(code string) string {
	if len(code) == 0 {
		return ""
	}
	switch code[0] {
	case '2':
		return "2xx"
	case '3':
		return "3xx"
	case '4':
		return "4xx"
	case '5':
		return "5xx"
	}
	return ""
}

// parseLine parses a single Prometheus text-format exposition line into its
// component parts. Returns false when the line cannot be parsed.
//
// Format: metric_name{label="value",...} value [timestamp]
// Or:     metric_name value [timestamp]   (no labels)
func parseLine(line string) (name string, labels map[string]string, value float64, ok bool) {
	labels = map[string]string{}

	// Split off the value (and optional timestamp) from the right.
	// The metric identifier part ends at the first space that is not inside {}.
	braceDepth := 0
	splitAt := -1
	for i, ch := range line {
		switch ch {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case ' ', '\t':
			if braceDepth == 0 {
				splitAt = i
				goto foundSplit
			}
		}
	}
foundSplit:
	if splitAt < 0 {
		return
	}

	metricPart := line[:splitAt]
	rest := strings.TrimSpace(line[splitAt+1:])

	// Parse value (ignore trailing timestamp).
	valStr := rest
	if idx := strings.IndexAny(rest, " \t"); idx >= 0 {
		valStr = rest[:idx]
	}
	v, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return
	}
	value = v

	// Split name from labels.
	bStart := strings.IndexByte(metricPart, '{')
	if bStart < 0 {
		name = metricPart
		ok = true
		return
	}

	name = metricPart[:bStart]
	labelStr := metricPart[bStart+1:]
	labelStr = strings.TrimSuffix(labelStr, "}")

	// Parse label pairs: key="value",…
	for _, pair := range splitLabelPairs(labelStr) {
		eqIdx := strings.IndexByte(pair, '=')
		if eqIdx < 0 {
			continue
		}
		k := strings.TrimSpace(pair[:eqIdx])
		v := strings.TrimSpace(pair[eqIdx+1:])
		v = strings.Trim(v, `"`)
		labels[k] = v
	}

	ok = true
	return
}

// splitLabelPairs splits a label string like `key="val",key2="v,al"` into
// individual key=value tokens, respecting quoted commas.
func splitLabelPairs(s string) []string {
	var pairs []string
	inQuote := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				pairs = append(pairs, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		pairs = append(pairs, s[start:])
	}
	return pairs
}
