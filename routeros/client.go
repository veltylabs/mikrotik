package routeros

import (
	"fmt"
	"time"

	goros "github.com/go-routeros/routeros/v3"
)

// Client is a Commander over a live API session.
type Client struct{
	client *goros.Client
}

const dialTimeout = 10 * time.Second

// Dial opens an API session. tls=true uses API-SSL without certificate
// verification (see docs/ARCHITECTURE.md "Connecting").
func Dial(address, user, password string, useTLS bool) (*Client, error) {
	var c *goros.Client
	var err error

	if useTLS {
		c, err = goros.DialTLSTimeout(address, user, password, nil, dialTimeout)
	} else {
		c, err = goros.DialTimeout(address, user, password, dialTimeout)
	}

	if err != nil {
		return nil, err
	}
	return &Client{client: c}, nil
}

func (c *Client) Run(sentence ...string) (Reply, error) {
	reply, err := c.client.RunArgs(sentence)
	if err != nil {
		// A *routeros.DeviceError (a !trap) is returned wrapped with the command path
		if _, ok := err.(*goros.DeviceError); ok {
			return Reply{}, fmt.Errorf("routeros %s: %w", sentence[0], err)
		}
		return Reply{}, err
	}

	r := Reply{
		Records: make([]Record, len(reply.Re)),
		Done:    Record(reply.Done.Map),
	}

	for i, re := range reply.Re {
		r.Records[i] = Record(re.Map)
	}

	return r, nil
}

func (c *Client) Close() error {
	if c.client == nil {
		return nil
	}
	// Note: go-routeros Close has no return value, so we just wrap it
	c.client.Close()
	return nil
}
