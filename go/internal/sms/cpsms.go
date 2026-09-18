package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// defaultSenderName is the alphanumeric sender shown on the recipient's phone.
// CPSMS requires it to be pre-approved on the account, so it is not a free
// string — override it via the DSN only if the account has another one.
const defaultSenderName = "Nathejk"

// CpsmsSender sends through CPSMS' v2 HTTP API. Same wire format as the
// implementation in the `tilmelding` repo, so one account and one approved
// sender name serve both.
type CpsmsSender struct {
	apiURL string
	apiKey string // pre-encoded HTTP Basic credential, used verbatim
	from   string
	client *http.Client
}

// NewCpsms builds a sender for host (e.g. "api.cpsms.dk") using apiKey as the
// Basic authorization value. from may be empty, in which case defaultSenderName
// is used.
func NewCpsms(host, apiKey, from string) (*CpsmsSender, error) {
	if host == "" {
		return nil, fmt.Errorf("cpsms: missing host in DSN")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("cpsms: missing API key in DSN (expected cpsms://<key>@%s)", host)
	}
	if from == "" {
		from = defaultSenderName
	}
	return &CpsmsSender{
		apiURL: "https://" + host + "/v2/send",
		apiKey: apiKey,
		from:   from,
		// A login PIN is worthless if it arrives a minute late, and the request is
		// made inline in an HTTP handler: bound it rather than letting a hung
		// provider hold a connection open.
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Send delivers message to the given number. to is expected in the normalized
// E.164 form the BFF uses internally ("+4530000001"); CPSMS wants digits only,
// including the country code, so the leading "+" and any separators are
// stripped here rather than at every call site.
func (s *CpsmsSender) Send(ctx context.Context, to, message string) error {
	recipient := digitsOnly(to)
	if recipient == "" {
		return fmt.Errorf("cpsms: empty recipient")
	}

	type request struct {
		To      string `json:"to"`
		From    string `json:"from"`
		Message string `json:"message"`
	}
	type apiError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	type response struct {
		Success []struct {
			To   string `json:"to"`
			Cost int    `json:"cost"`
		} `json:"success"`
		Error *apiError `json:"error"`
	}

	body, err := json.Marshal(request{To: recipient, From: s.from, Message: message})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Basic "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var out response
	if decErr := json.NewDecoder(resp.Body).Decode(&out); decErr != nil {
		// A non-2xx with an unparseable body (a proxy error page, say) must not be
		// reported as success, so the status is part of the error.
		return fmt.Errorf("cpsms: status %d: %w", resp.StatusCode, decErr)
	}
	if out.Error != nil {
		return fmt.Errorf("cpsms: error %d: %q", out.Error.Code, out.Error.Message)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("cpsms: status %d", resp.StatusCode)
	}
	if len(out.Success) == 0 {
		return fmt.Errorf("cpsms: provider accepted no recipient")
	}
	return nil
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
