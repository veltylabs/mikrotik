package mikrotik

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

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

// Gateway is a network.Gateway over one router, reached through a reconnecting
// Session. It detects the RouterOS version on every new connection.
type Gateway struct {
	session    *routeros.Session
	mu         sync.Mutex
	dialect    network.Gateway
	version    string
	generation uint64
}

// New builds a Gateway over session (Open uses it).
func New(session *routeros.Session) *Gateway {
	return &Gateway{
		session: session,
	}
}

// Open validates rawURL (see EnvRouterURL) and returns a Gateway WITHOUT dialing.
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

	dial := func() (routeros.Conn, error) {
		return routeros.Dial(host, user, password, useTLS)
	}

	return New(routeros.NewSession(dial)), nil
}

func (g *Gateway) current() (network.Gateway, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Re-detect when there is no cached dialect, when the connection is down
	// (the next command will dial a router that may have been upgraded), or when
	// a new connection was made since the dialect was chosen.
	if g.dialect != nil && g.session.Live() && g.generation == g.session.Generation() {
		return g.dialect, nil
	}

	reply, err := g.session.Run(v6.PathResource + v6.CmdPrint)
	if err != nil {
		return nil, err
	}

	// Read generation AFTER the print, since the print might have dialed and bumped it
	newGen := g.session.Generation()

	if len(reply.Records) == 0 {
		return nil, fmt.Errorf("mikrotik: empty reply from %s%s", v6.PathResource, v6.CmdPrint)
	}

	version := reply.Records[0][propVersion]

	dialect, err := dialectFor(version, g.session)
	if err != nil {
		return nil, err
	}

	g.dialect = dialect
	g.version = version
	g.generation = newGen

	return g.dialect, nil
}

func dialectFor(version string, c routeros.Commander) (network.Gateway, error) {
	if strings.HasPrefix(version, "6.") {
		return v6.New(c), nil
	} else if strings.HasPrefix(version, "7.") {
		return v7.New(c), nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedVersion, version)
}

func (g *Gateway) Plan(d network.Desired) (network.Plan, error) {
	dialect, err := g.current()
	if err != nil {
		return network.Plan{}, err
	}
	return dialect.Plan(d)
}

func (g *Gateway) Apply(d network.Desired, expected network.Fingerprint) (network.Plan, error) {
	dialect, err := g.current()
	if err != nil {
		return network.Plan{}, err
	}
	return dialect.Apply(d, expected)
}

func (g *Gateway) Connections() ([]network.Connection, error) {
	dialect, err := g.current()
	if err != nil {
		return nil, err
	}
	return dialect.Connections()
}

func (g *Gateway) Discover() ([]network.Discovered, error) {
	dialect, err := g.current()
	if err != nil {
		return nil, err
	}
	return dialect.Discover()
}

// Version reads the version of the current connection (dialing if needed).
func (g *Gateway) Version() (string, error) {
	_, err := g.current()
	if err != nil {
		return "", err
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	return g.version, nil
}

func (g *Gateway) Close() error {
	return g.session.Close()
}

var _ network.Gateway = (*Gateway)(nil)
