package apilog

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// Transport wraps base so every RoundTrip is captured into r. It
// reads the request body once (and re-installs it for the wire) and
// reads the response body once (then re-installs a buffered copy so
// downstream consumers still see the original bytes). Sensitive
// headers and PII keys are scrubbed before write.
//
// When r is nil or disabled, the returned RoundTripper is base
// untouched — the recorder adds zero overhead in that path.
func (r *Recorder) Transport(base http.RoundTripper) http.RoundTripper {
	if r == nil || r.disabled {
		if base == nil {
			return http.DefaultTransport
		}
		return base
	}
	if base == nil {
		base = http.DefaultTransport
	}
	return &recorderTransport{base: base, rec: r}
}

type recorderTransport struct {
	base http.RoundTripper
	rec  *Recorder
}

func (t *recorderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Capture (and re-install) the request body.
	var reqBody []byte
	if req.Body != nil {
		buf, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err == nil {
			reqBody = buf
			req.Body = io.NopCloser(bytes.NewReader(buf))
		} else {
			req.Body = io.NopCloser(bytes.NewReader(nil))
		}
	}

	resp, err := t.base.RoundTrip(req)

	entry := Entry{
		Time:           start,
		Method:         req.Method,
		URL:            req.URL.String(),
		Path:           req.URL.Path,
		DurationMS:     time.Since(start).Milliseconds(),
		RequestHeaders: RedactHeaders(req.Header),
		RequestBody:    string(RedactJSONBody(CapBody(reqBody))),
	}
	if err != nil {
		entry.Err = err.Error()
		t.rec.Record(entry)
		return resp, err
	}

	// Capture (and re-install) the response body.
	var respBody []byte
	if resp.Body != nil {
		buf, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr == nil {
			respBody = buf
		}
		resp.Body = io.NopCloser(bytes.NewReader(buf))
	}

	entry.Status = resp.StatusCode
	entry.ResponseHeaders = RedactHeaders(resp.Header)
	entry.ResponseBody = string(RedactJSONBody(CapBody(respBody)))
	t.rec.Record(entry)
	return resp, nil
}
