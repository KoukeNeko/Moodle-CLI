package moodle

import (
	"context"
	"encoding/json"
)

// RawCaller sends an arbitrary web service call and hands the reply back
// unread.
//
// Everything else in this package maps Moodle's shapes onto this project's
// own, so upstream field names stop at the adapter. This one deliberately does
// not: there is no typed shape for a function nobody has looked at, and
// inventing one would be a promise that cannot be kept.
type RawCaller struct {
	client *Client
	token  string
}

// NewRawCaller builds the escape hatch's transport.
func NewRawCaller(client *Client, token string) *RawCaller {
	return &RawCaller{client: client, token: token}
}

func (c *RawCaller) Call(ctx context.Context, function string, params map[string]any) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.client.Call(ctx, c.token, function, Params(params), &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
