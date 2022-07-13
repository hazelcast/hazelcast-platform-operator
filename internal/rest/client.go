package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	BaseURL *url.URL
	client  *http.Client
}

func (c *Client) NewRequest(method, url string, body interface{}) (*http.Request, error) {
	u, err := c.BaseURL.Parse(url)
	if err != nil {
		return nil, err
	}

	header := make(http.Header)
	var r io.Reader

	// serialize the request body
	if body != nil {
		switch v := body.(type) {
		case nil:
			// do nothing
		case io.Reader:
			r = v
		case string:
			r = strings.NewReader(v)
		default:
			buf := &bytes.Buffer{}
			enc := json.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(body); err != nil {
				return nil, err
			}
			header.Set("Content-Type", "application/json")
			r = buf
		}
	}

	// prepare new request
	req, err := http.NewRequest(method, u.String(), r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header = header
	}

	return req, nil
}

func (c *Client) Do(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	// send the request
	resp, err := c.client.Do(req)
	if err != nil {
		// If we got an error, and the context has been canceled,
		// the context's error is probably more useful.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, err
	}
	defer resp.Body.Close()

	// check for http status errors
	err = checkResponse(resp)
	if err != nil {
		return resp, err
	}

	// decode the body if needed
	switch v := v.(type) {
	case nil:
		// do nothing
	case io.Writer:
		_, err = io.Copy(v, resp.Body)
	default:
		d := json.NewDecoder(resp.Body)
		if err2 := d.Decode(v); err2 != nil {
			// ignore EOF errors caused by empty response body
			if err2 != io.EOF {
				err = err2
			}
		}
	}

	return resp, err
}

func checkResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}
	return &ErrorResponse{Response: r}
}

type ErrorResponse struct {
	Response *http.Response
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d", r.Response.Request.Method, r.Response.Request.URL, r.Response.StatusCode)
}
