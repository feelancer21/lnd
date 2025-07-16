package routing

import (
	"errors"
	"sync"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
)

const (
	// Parts per million (ppm) for cost calculations
	rateParts = 1e6

	// Maximum rate in ppm to prevent overflow in calculations.
	maxRatePpm = 10 * rateParts

	// Minimum fee because Dijkstra requires a non-zero fee.
	minCost = 0
)

var (
	// errNamespaceNotFound is returned when a requested namespace does not
	// exist in the ImputedCostManager.
	errNamespaceNotFound = errors.New("imputed cost namespace not found")

	// errInsufficientCostLimit is returned when the imputed cost exceeds
	// the specified limit.
	errInsufficientCostLimit = errors.New("imputed cost exceeds limit")

	// errInsufficientAttemptCostLimit is returned when the imputed attempt
	// cost exceeds the specified limit.
	errInsufficientAttemptCostLimit = errors.New("imputed attempt cost " +
		"exceeds limit")
)

// imputedCostModel is an interface that provides imputed cost calculations
// for payments between node pairs. It supports two types of cost: cost that
// only apply when payments succeed, and attempt cost that apply regardless
// of payment outcome.
type imputedCostModel interface {
	// getCost returns the imputed cost in millisatoshis that
	// apply only when a payment from fromNode to toNode succeeds for the
	// given amount.
	getCost(fromNode, toNode route.Vertex,
		amount lnwire.MilliSatoshi) lnwire.MilliSatoshi

	// getAttemptCost returns the imputed attempt cost in
	// millisatoshis that apply regardless of whether a payment from
	// fromNode to toNode succeeds or fails for the given amount.
	getAttemptCost(fromNode, toNode route.Vertex,
		amount lnwire.MilliSatoshi) lnwire.MilliSatoshi
}

type ImputedCostControl struct {
	model            imputedCostModel
	costLimit        fn.Option[lnwire.MilliSatoshi]
	attemptCostLimit fn.Option[lnwire.MilliSatoshi]
}

func (c *ImputedCostControl) processPair(fromNode, toNode route.Vertex,
	amount lnwire.MilliSatoshi, totalFee int64, absoluteAttemptCost float64,
	imputedCost, imputedAttemptCost *lnwire.MilliSatoshi) error {

	// Calculate total cost including imputed cost.
	costPair := c.model.getCost(fromNode, toNode, amount)

	// Check if cost limit is exceeded.
	if fn.MapOptionZ(c.costLimit, func(l lnwire.MilliSatoshi) bool {
		return costPair+lnwire.MilliSatoshi(totalFee)+
			*imputedCost > l
	}) {
		return errInsufficientCostLimit
	}

	// Calculate total attempt cost.
	attemptCostPair := c.model.getAttemptCost(fromNode, toNode, amount)

	// Check if attempt cost limit is exceeded.
	if fn.MapOptionZ(c.attemptCostLimit, func(l lnwire.MilliSatoshi) bool {
		return attemptCostPair+lnwire.MilliSatoshi(absoluteAttemptCost)+
			*imputedAttemptCost > l
	}) {
		return errInsufficientAttemptCostLimit
	}

	*imputedCost += costPair
	*imputedAttemptCost += attemptCostPair
	return nil
}

// ImputedCostParameters defines the cost parameters for a node pair, mirroring
// the structure defined in router.proto.
type imputedCostParameters struct {
	// costRatePpm is the imputed cost rate in parts per million (ppm) of
	// the amount sent. This cost only incurs if the payment is successful.
	costRatePpm int64

	// costBaseMsat is the base imputed cost in millisatoshis. This cost
	// only incurs if the payment is successful.
	costBaseMsat int64

	// attemptCostRatePpm is the attempt cost rate in parts per million
	// (ppm) of the amount sent. This cost incurs regardless of whether
	// the payment is successful or not.
	attemptCostRatePpm int64

	// attemptCostBaseMsat is the base attempt cost in millisatoshis. This
	// cost incurs regardless of whether the payment is successful or not.
	attemptCostBaseMsat int64
}

// imputedCostNamespace represents an imputed cost namespace that contains
// default parameters and specific node pair configurations.
type imputedCostNamespace struct {
	// defaultParams are the default cost parameters applied to all
	// node pairs that do not have explicitly defined parameters.
	defaultParams imputedCostParameters

	// pairParams is a map of node pairs to their specific cost parameters.
	// The key is constructed from the FromNode and ToNode vertices.
	pairParams map[DirectedNodePair]imputedCostParameters
}

func (c *imputedCostNamespace) getNodePairParams(fromNode,
	toNode route.Vertex) imputedCostParameters {

	pair := NewDirectedNodePair(fromNode, toNode)
	if params, ok := c.pairParams[pair]; ok {
		return params
	}
	return c.defaultParams
}

// linearCostModel implements the imputedCostModel interface using a linear
// cost calculation model based on base cost and rates.
type linearCostModel struct {
	ns *imputedCostNamespace
}

// A compile time check to ensure LinearCostModel implements the
// imputedCostModel interface.
var _ imputedCostModel = (*linearCostModel)(nil)

func calcCost(baseMsat, ratePpm int64,
	amount lnwire.MilliSatoshi) lnwire.MilliSatoshi {

	if ratePpm > maxRatePpm {
		ratePpm = maxRatePpm
	}

	cost := (ratePpm*int64(amount))/rateParts + baseMsat
	if cost < minCost {
		cost = minCost
	}
	return lnwire.MilliSatoshi(cost)
}

func (l *linearCostModel) getCost(fromNode, toNode route.Vertex,
	amount lnwire.MilliSatoshi) lnwire.MilliSatoshi {

	p := l.ns.getNodePairParams(fromNode, toNode)

	return calcCost(p.costBaseMsat, p.costRatePpm, amount)
}
func (l *linearCostModel) getAttemptCost(fromNode, toNode route.Vertex,
	amount lnwire.MilliSatoshi) lnwire.MilliSatoshi {

	p := l.ns.getNodePairParams(fromNode, toNode)

	return calcCost(p.attemptCostBaseMsat, p.attemptCostRatePpm, amount)
}

// ImputedCostManager manages imputed cost namespaces.
type ImputedCostManager struct {
	namespaces map[string]*imputedCostNamespace

	// mu protects access to the namespaces map and ensures thread safety
	// for all data manipulation operations.
	mu sync.RWMutex
}

// NewImputedCostManager creates a new ImputedCostManager instance with an
// empty set of namespaces.
func NewImputedCostManager() *ImputedCostManager {
	return &ImputedCostManager{
		namespaces: make(map[string]*imputedCostNamespace),
	}
}

// GetNamespacedModel returns an imputedCostModel initialized with the
// specified namespace. Returns an error if the namespace does not exist.
func (m *ImputedCostManager) getNamespacedModel(ns string) (
	imputedCostModel, error) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	if namespace, ok := m.namespaces[ns]; ok {
		// Return a new LinearCostModel instance for this namespace
		return &linearCostModel{ns: namespace}, nil
	}

	return nil, errNamespaceNotFound
}

func (m *ImputedCostManager) GetNamespacedControl(ns string,
	costLimit, attemptCostLimit fn.Option[lnwire.MilliSatoshi]) (
	*ImputedCostControl, error) {

	model, err := m.getNamespacedModel(ns)
	if err != nil {
		return nil, err
	}

	return &ImputedCostControl{
		model:            model,
		costLimit:        costLimit,
		attemptCostLimit: attemptCostLimit,
	}, nil
}
