package mikrotik

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/veltylabs/mikrotik/routeros"
	"github.com/veltylabs/mikrotik/v6"
	"github.com/veltylabs/mikrotik/v7"
	"webtyp.com/network"
)

const (
	EnvRouterURL = "ROUTER_URL"
	SchemeAPI    = "routeros"
	SchemeAPITLS = "routeros+tls"
	portAPI      = "8728"
	portAPITLS   = "8729"

	propVersion = "version" // /system/resource property with the RouterOS version
)

var (
	ErrScheme             = errors.New("mikrotik: ROUTER_URL scheme must be routeros:// or routeros+tls://")
	ErrUnsupportedVersion = errors.New("mikrotik: unsupported RouterOS version")
)

// Gateway is a network.Gateway connected to one router.
type Gateway struct {
	network.Gateway
	version string
	conn    *routeros.Client
}

// Open parses rawURL (see EnvRouterURL), connects, reads the RouterOS
// version from /system/resource and selects the v6 or v7 dialect.
func Open(rawURL string) (*Gateway, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	useTLS := false
	port := ""

	switch u.Scheme {
	case SchemeAPI:
		useTLS = false
		port = portAPI
	case SchemeAPITLS:
		useTLS = true
		port = portAPITLS
	default:
		return nil, ErrScheme
	}

	if u.User == nil || u.User.Username() == "" {
		return nil, errors.New("mikrotik: ROUTER_URL has no user")
	}

	user := u.User.Username()
	password, _ := u.User.Password()

	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":" + port
	}

	client, err := routeros.Dial(host, user, password, useTLS)
	if err != nil {
		return nil, err
	}

	reply, err := client.Run(v6.PathResource + v6.CmdPrint)
	if err != nil {
		client.Close()
		return nil, err
	}

	if len(reply.Records) == 0 {
		client.Close()
		return nil, fmt.Errorf("mikrotik: empty reply from %s%s", v6.PathResource, v6.CmdPrint)
	}

	version := reply.Records[0][propVersion]

	dialect, err := dialectFor(version, client)
	if err != nil {
		client.Close()
		return nil, err
	}

	return &Gateway{
		Gateway: dialect,
		version: version,
		conn:    client,
	}, nil
}

func dialectFor(version string, c routeros.Commander) (network.Gateway, error) {
	if strings.HasPrefix(version, "6.") {
		return v6.New(c), nil
	} else if strings.HasPrefix(version, "7.") {
		return v7.New(c), nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedVersion, version)
}

// Version is the RouterOS version string reported by the router, e.g. "7.20.2 (stable)".
func (g *Gateway) Version() string {
	return g.version
}

func (g *Gateway) Close() error {
	return g.conn.Close()
}
