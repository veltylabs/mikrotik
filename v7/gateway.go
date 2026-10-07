package v7

import (
	"github.com/veltylabs/mikrotik/routeros"
	"github.com/veltylabs/mikrotik/v6"
	"webtyp.com/network"
)

// Gateway implements network.Gateway for RouterOS 7.x. Every command used
// by the network contract is identical in RouterOS 6 and 7, so it delegates
// to v6; a difference is added here as a method override, never as a
// version check inside v6.
type Gateway struct{ *v6.Gateway }

func New(c routeros.Commander) *Gateway { return &Gateway{v6.New(c)} }

var _ network.Gateway = (*Gateway)(nil)
