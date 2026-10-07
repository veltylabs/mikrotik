package v6

import (
	"strings"

	"webtyp.com/network"
)

func (g *Gateway) Apply(d network.Desired, expected network.Fingerprint) (network.Plan, error) {
	pr, err := g.makePlan(d)
	if err != nil {
		return network.Plan{}, err
	}

	if pr.Fingerprint != expected {
		return network.Plan{}, network.ErrPlanStale
	}

	if len(pr.Conflicts) > 0 {
		return network.Plan{}, network.ErrConflicts
	}

	// Compute anchor once for filter and nat
	snap, err := g.read(d.Settings)
	if err != nil {
		return network.Plan{}, err
	}

	var filterAnchor string
	for _, r := range snap.Filters {
		if r["chain"] == "forward" {
			filterAnchor = r[PropID]
			break
		}
	}

	var natAnchor string
	for _, r := range snap.NATs {
		if r["chain"] == "dstnat" {
			natAnchor = r[PropID]
			break
		}
	}

	for _, a := range pr.Actions {
		runArgs := a.Run

		if a.Change.Kind == network.ChangeAdd {
			if strings.HasPrefix(runArgs[0], PathFilter) {
				if filterAnchor != "" {
					runArgs = append(runArgs, "=place-before="+filterAnchor)
				}
			} else if strings.HasPrefix(runArgs[0], PathNAT) {
				if natAnchor != "" {
					runArgs = append(runArgs, "=place-before="+natAnchor)
				}
			}
		}

		_, err := g.C.Run(runArgs...)
		if err != nil {
			return network.Plan{}, err
		}
	}

	return pr.Plan, nil
}
