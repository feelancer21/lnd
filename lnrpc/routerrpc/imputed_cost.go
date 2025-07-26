package routerrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing"
	"github.com/lightningnetwork/lnd/routing/route"
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

func (s *Server) XImportImputedCosts(_ context.Context,
	req *ImportImputedCostsRequest) (*ImportImputedCostsResponse, error) {

	data := make(map[string]*ImputedCostNamespace)
	names := make(fn.Set[string])

	for _, ns := range req.Namespaces {
		if ok := names.Contains(ns.Name); ok {
			return nil, fmt.Errorf("found duplicate for "+
				"namespace %v", ns.Name)
		}

		names.Add(ns.Name)
		data[ns.Name] = ns
	}

	importNs := func(name string, ns *routing.ImputedCostNamespace) error {
		in, ok := data[name]

		// This should not happen because of the construction of data
		// names.
		if !ok || in == nil {
			return fmt.Errorf("unexpected error occurred")
		}

		if in.DefaultParameters != nil {
			defaults, err := unmarshalImputedParams(
				in.DefaultParameters,
			)
			if err != nil {
				return err
			}
			ns.DefaultParams = *defaults
		}

		err := unmarshalImputedPairs(in.Pairs, ns.PairParams)
		if err != nil {
			return err
		}

		return nil
	}

	if err := s.cfg.RouterBackend.ImputedCostManager.ImportNamespaces(
		names, req.Reset_, importNs,
	); err != nil {
		return nil, err
	}

	return &ImportImputedCostsResponse{}, nil
}

func unmarshalImputedPairs(pairs []*ImputedCostPair,
	res map[routing.DirectedNodePair]routing.ImputedCostParameters) error {

	for _, pair := range pairs {
		from, err := route.NewVertexFromBytes(pair.NodeFrom)
		if err != nil {
			return fmt.Errorf("invalid node: %x", pair.NodeFrom)
		}
		to, err := route.NewVertexFromBytes(pair.NodeTo)
		if err != nil {
			return fmt.Errorf("invalid node: %x", pair.NodeTo)
		}

		params, err := unmarshalImputedParams(pair.Parameters)
		if err != nil {
			return err
		}

		directed := routing.DirectedNodePair{From: from, To: to}
		if params == nil {
			delete(res, directed)
		} else {
			res[directed] = *params
		}
	}

	return nil
}

func unmarshalImputedParams(
	params *ImputedCostParameters) (*routing.ImputedCostParameters, error) {

	if params == nil {
		return nil, nil
	}

	switch {
	case params.AttemptCostBaseMsat < 0:
		return nil, fmt.Errorf("invalid attempt cost base msat: %v",
			params.AttemptCostBaseMsat)

	case params.AttemptCostRatePpm < 0:
		return nil, fmt.Errorf("invalid attempt cost rate ppm: %v",
			params.AttemptCostRatePpm)

	case params.CostBaseMsat < 0:
		return nil, fmt.Errorf("invalid cost base msat: %v",
			params.CostBaseMsat)

	case params.CostRatePpm < 0:
		return nil, fmt.Errorf("invalid cost rate ppm: %v",
			params.CostRatePpm)
	}

	return &routing.ImputedCostParameters{
		CostRatePpm:         int64(params.CostRatePpm),
		CostBaseMsat:        int64(params.CostBaseMsat),
		AttemptCostRatePpm:  int64(params.AttemptCostRatePpm),
		AttemptCostBaseMsat: int64(params.AttemptCostBaseMsat),
	}, nil
}

func (s *Server) XQueryImputedCosts(_ context.Context,
	req *QueryImputedCostsRequest) (*QueryImputedCostsResponse, error) {

	names := fn.NewSet(req.Namespaces...)
	res := make([]*ImputedCostNamespace, 0, names.Size())

	if err := s.cfg.RouterBackend.ImputedCostManager.QueryNamespaces(
		names, func(name string, ns *routing.ImputedCostNamespace) {
			res = append(res, marshalImputedNamespace(name, ns))
		},
	); err != nil {
		return nil, err
	}

	return &QueryImputedCostsResponse{Namespaces: res}, nil
}

func marshalImputedNamespace(name string,
	ns *routing.ImputedCostNamespace) *ImputedCostNamespace {

	costPairs := make([]*ImputedCostPair, 0, len(ns.PairParams))
	for vertices, params := range ns.PairParams {
		pairParam := &ImputedCostPair{
			NodeFrom:   vertices.From[:],
			NodeTo:     vertices.To[:],
			Parameters: marshalImputedParams(&params),
		}
		costPairs = append(costPairs, pairParam)
	}

	return &ImputedCostNamespace{
		Name:              name,
		DefaultParameters: marshalImputedParams(&ns.DefaultParams),
		Pairs:             costPairs,
	}
}

func marshalImputedParams(
	params *routing.ImputedCostParameters) *ImputedCostParameters {

	if params == nil {
		return nil
	}

	return &ImputedCostParameters{
		CostRatePpm:         int32(params.CostRatePpm),
		CostBaseMsat:        int32(params.CostBaseMsat),
		AttemptCostRatePpm:  int32(params.AttemptCostRatePpm),
		AttemptCostBaseMsat: int32(params.AttemptCostBaseMsat),
	}
}

func (s *Server) XDeleteImputedCosts(_ context.Context,
	req *DeleteImputedCostsRequest) (*DeleteImputedCostsResponse, error) {

	if len(req.Namespaces) == 0 {
		return nil, errors.New("at least one namespace required")
	}

	names := fn.NewSet(req.Namespaces...)

	err := s.cfg.RouterBackend.ImputedCostManager.DeleteNamespaces(names)
	if err != nil {
		return nil, err
	}

	return &DeleteImputedCostsResponse{}, nil
}
