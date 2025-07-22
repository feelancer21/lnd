package routerrpc

import (
	"fmt"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing"
)

// parseImputedCostRestr converts a protobuf ImputedCostRestriction to the
// internal routing type. Returns nil, nil if the input is nil (not an error).
func parseImputedCostRestr(restr *lnrpc.ImputedCostRestriction) (
	*routing.ImputedCostRestriction, error) {

	if restr == nil {
		return nil, nil
	}

	if restr.TotalCostLimitMsat < 0 {
		return nil, fmt.Errorf("total cost limit must be non-negative:"+
			" %d", restr.TotalCostLimitMsat)
	}

	// Optional cost limit. Zero is interpreted as no limit.
	var limit routing.OptionalLimit
	if restr.TotalCostLimitMsat > 0 {
		limit = fn.Some(lnwire.MilliSatoshi(restr.TotalCostLimitMsat))
	}

	return &routing.ImputedCostRestriction{
		Namespace: restr.Namespace,
		CostLimit: limit,
	}, nil
}
