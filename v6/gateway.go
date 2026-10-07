package v6

import (
	"github.com/veltylabs/mikrotik/routeros"
	"webtyp.com/network"
)

// Gateway implements network.Gateway for RouterOS 6.x.
type Gateway struct{ C routeros.Commander }

func New(c routeros.Commander) *Gateway {
	return &Gateway{C: c}
}

var _ network.Gateway = (*Gateway)(nil)
