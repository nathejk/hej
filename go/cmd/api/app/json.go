package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// maxRequestBody caps the size of a JSON request body (1 MiB).
const maxRequestBody = 1_048_576

// WriteJSON marshals data and writes it as an application/json response with the
// given status code and optional extra headers.
func (a *JsonApi) WriteJSON(w http.ResponseWriter, status int, data any, headers http.Header) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}
	js = append(js, '\n')

	for key, values := range headers {
		w.Header()[key] = values
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(js)
	return err
}

// ReadJSON decodes a single JSON value from the request body into dst,
// rejecting unknown fields and trailing content.
func (a *JsonApi) ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("body must only contain a single JSON value")
	}
	return nil
}
