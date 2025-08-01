package routing

import (
	"errors"
	"fmt"
	"sync"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
)

const (
	// rateParts defines parts per million (ppm) for cost calculations.
	rateParts = 1e6

	// maxRatePpm defines the maximum rate in ppm to prevent overflow in
	// calculations.
	maxRatePpm = 10 * rateParts

	// minCost defines the minimum cost value because Dijkstra requires a
	// non-negative cost.
	minCost = 0
)

var (
	// errInsufficientCostLimit is returned when the imputed cost exceeds
	// the specified limit.
	errInsufficientCostLimit = errors.New("imputed cost exceeds limit")
)

type OptionalLimit = fn.Option[lnwire.MilliSatoshi]

// imputedCostModel is an interface that provides imputed cost calculations
// for payments between node pairs. It supports two types of costs: costs that
// only apply when payments succeed, and attempt costs that apply regardless
// of payment outcome.
type imputedCostModel interface {
	// getCost returns the imputed costs in millisatoshis that
	// apply only when a payment from fromNode to toNode succeeds for the
	// given amount.
	getCost(fromNode, toNode route.Vertex,
		amount lnwire.MilliSatoshi) lnwire.MilliSatoshi

	// getAttemptCost returns the imputed attempt costs in
	// millisatoshis that apply regardless of whether a payment from
	// fromNode to toNode succeeds or fails for the given amount.
	getAttemptCost(fromNode, toNode route.Vertex,
		amount lnwire.MilliSatoshi) lnwire.MilliSatoshi
}

// imputedDistInfo holds the imputed cost information for the whole path from a
// specific vertex to the target.
type imputedDistInfo struct {
	// cost represents the total imputed costs that apply only when payments
	// succeed.
	cost lnwire.MilliSatoshi

	// attemptCost represents the total attempt costs that apply regardless
	// of payment outcome.
	attemptCost lnwire.MilliSatoshi
}

// getDist returns the probability-based distance with the imputed cost to the
// target.
func (d *imputedDistInfo) getDist(probability float64) int64 {
	// NOTE:
	// As long as cost and attemptCost are not decreasing from one vertex
	// to the next and the probability is not increasing, we can conclude
	// the delta of the distance is always positive.

	//    A_2 + C_2 / P_2 - A_1 - C_1 / P_1
	// = (A_2 - A_1) + C_2 / P_2 - C_1 / P_1

	// A_2 - A_1 cannot be negative, and the second term because of
	// C_2 / P_2 - C_1 / P_1 >= 0 <=> C_2 * P_1 >= C_1 * P_2
	return getProbabilityBasedDist(
		int64(d.cost), probability, float64(d.attemptCost),
	)
}

// ImputedCostControl controls the imputed cost calculations and limits
// during one pathfinding attempt.
type ImputedCostControl struct {
	// model is the cost model used for calculating the imputed costs.
	model imputedCostModel

	// costLimit is the optional cost limit in millisatoshis.
	costLimit OptionalLimit
}

// expandDistInfo expands the imputed cost information for the path from
// fromNode to toNode with the given amount. It returns an error if the sum of
// totalFees and imputed costs exceeds the optional cost limit.
func (c *ImputedCostControl) expandDistInfo(fromNode, toNode route.Vertex,
	amount lnwire.MilliSatoshi, totalFee int64,
	toDistInfo imputedDistInfo) (imputedDistInfo, error) {

	res := toDistInfo

	// Calculate total costs including imputed costs.
	res.cost += c.model.getCost(fromNode, toNode, amount)

	// Check if totalFee plus imputed costs exceed the cost limit.
	if fn.MapOptionZ(c.costLimit, func(l lnwire.MilliSatoshi) bool {
		return res.cost+lnwire.MilliSatoshi(totalFee) > l
	}) {

		return imputedDistInfo{}, errInsufficientCostLimit
	}

	res.attemptCost += c.model.getAttemptCost(fromNode, toNode, amount)

	return res, nil
}

// ImputedCostParameters defines the cost parameters for a node pair, mirroring
// the structure defined in router.proto.
type ImputedCostParameters struct {
	// CostRatePpm is the imputed cost rate in parts per million (ppm) of
	// the amount sent. This cost only applies if the payment is
	// successful.
	CostRatePpm int64

	// CostBaseMsat is the base imputed cost in millisatoshis. This cost
	// only applies if the payment is successful.
	CostBaseMsat int64

	// AttemptCostRatePpm is the attempt cost rate in parts per million
	// (ppm) of the amount sent. This cost applies regardless of whether
	// the payment is successful or not.
	AttemptCostRatePpm int64

	// AttemptCostBaseMsat is the base attempt cost in millisatoshis. This
	// cost applies regardless of whether the payment is successful or not.
	AttemptCostBaseMsat int64
}

// ImputedCostNamespace represents an imputed cost namespace that contains
// default parameters and specific node pair configurations.
type ImputedCostNamespace struct {
	// defaultParams are the default cost parameters applied to all
	// node pairs that do not have explicitly defined parameters.
	DefaultParams ImputedCostParameters

	// pairParams is a map of node pairs to their specific cost parameters.
	// The key is constructed from the FromNode and ToNode vertices.
	PairParams map[DirectedNodePair]ImputedCostParameters
}

// newNamespaceEmpty creates a new ImputedCostNamespace with empty
// PairParams and the default parameters set to zero values.
func newNamespaceEmpty() *ImputedCostNamespace {
	return &ImputedCostNamespace{
		PairParams: make(map[DirectedNodePair]ImputedCostParameters),
	}
}

// getNodePairParams retrieves the cost parameters for the specific node pair.
// If there are no specific parameters for the pair, it returns the default
// parameters.
func (c *ImputedCostNamespace) getNodePairParams(fromNode,
	toNode route.Vertex) ImputedCostParameters {

	pair := NewDirectedNodePair(fromNode, toNode)
	if params, ok := c.PairParams[pair]; ok {
		return params
	}

	return c.DefaultParams
}

// Copy creates a deep copy of the ImputedCostNamespace.
func (c *ImputedCostNamespace) Copy() *ImputedCostNamespace {
	// Create a new namespace with the same default parameters.
	copyNs := newNamespaceEmpty()
	copyNs.DefaultParams = c.DefaultParams

	// Copy over the pair-specific parameters.
	for pair, params := range c.PairParams {
		copyNs.PairParams[pair] = params
	}

	return copyNs
}

// linearCostModel implements the imputedCostModel interface using a linear
// cost calculation model based on base costs and rates.
type linearCostModel struct {
	// ns is the namespace containing the cost parameters.
	ns *ImputedCostNamespace
}

// A compile time check to ensure linearCostModel implements the
// imputedCostModel interface.
var _ imputedCostModel = (*linearCostModel)(nil)

// calcCost calculates the imputed cost based on the base cost, rate, and
// amount. It ensures the costs are non-negative and do not exceed the maximum
// rate of 10M ppm.
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

// getCost returns the imputed costs in millisatoshis that apply only when the
// given amount moves from fromNode to toNode successfully.
func (l *linearCostModel) getCost(fromNode, toNode route.Vertex,
	amount lnwire.MilliSatoshi) lnwire.MilliSatoshi {

	p := l.ns.getNodePairParams(fromNode, toNode)

	return calcCost(p.CostBaseMsat, p.CostRatePpm, amount)
}

// getAttemptCost returns the imputed attempt costs in millisatoshis that apply
// regardless of whether the amount moves from fromNode to toNode successfully.
func (l *linearCostModel) getAttemptCost(fromNode, toNode route.Vertex,
	amount lnwire.MilliSatoshi) lnwire.MilliSatoshi {

	p := l.ns.getNodePairParams(fromNode, toNode)

	return calcCost(p.AttemptCostBaseMsat, p.AttemptCostRatePpm, amount)
}

// ImputedCostRestriction defines a restriction for imputed costs during a
// route request attempt.
type ImputedCostRestriction struct {
	// Namespace used for calculating the imputed costs.
	Namespace string

	// CostLimit is an optional limit for the imputed costs in msat.
	CostLimit OptionalLimit
}

// ImputedCostManager manages imputed cost namespaces and provides thread-safe
// access to cost models and controls.
type ImputedCostManager struct {
	// selfNode is the vertex representing this node in the network graph.
	selfNode route.Vertex

	// namespaces is a map of namespace names to their cost configurations.
	namespaces map[string]*ImputedCostNamespace

	// mu protects access to the namespaces map and ensures thread safety
	// for all data manipulation operations.
	mu sync.Mutex

	// fetchPayment fetches the payment corresponding to the given payment
	// hash.
	fetchPayment func(paymentHash lntypes.Hash) (DBMPPayment, error)
}

// NewImputedCostManager creates a new ImputedCostManager instance with an
// empty set of namespaces.
func NewImputedCostManager(selfNode route.Vertex, fetchPayment func(
	paymentHash lntypes.Hash) (DBMPPayment, error)) *ImputedCostManager {

	return &ImputedCostManager{
		selfNode:     selfNode,
		namespaces:   make(map[string]*ImputedCostNamespace),
		fetchPayment: fetchPayment,
	}
}

// getNamespace is a helper function for extracting a namespace. It returns
// an appropriate error if the namespace does not exist.
// NOTE: This function is not thread-safe and should only be called when
// the caller already holds the mutex.
func (m *ImputedCostManager) getNamespace(name string) (
	*ImputedCostNamespace, error) {

	if ns, ok := m.namespaces[name]; ok {
		return ns, nil
	}

	return nil, fmt.Errorf("imputed cost namespace not found: %v", name)
}

// getNamespacedModel returns an imputedCostModel initialized with the
// specified namespace. Returns an error if the namespace does not exist.
func (m *ImputedCostManager) getNamespacedModel(name string) (
	imputedCostModel, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	ns, err := m.getNamespace(name)
	if err != nil {
		return nil, err
	}

	// Return a new linearCostModel instance for this namespace.
	return &linearCostModel{ns: ns}, nil
}

// GetControl returns an ImputedCostControl for the specified
// restriction containing the namespace and cost limit.
func (m *ImputedCostManager) GetControl(
	restr *ImputedCostRestriction) (*ImputedCostControl, error) {

	if restr == nil {
		return nil, nil
	}

	model, err := m.getNamespacedModel(restr.Namespace)
	if err != nil {
		return nil, err
	}

	return &ImputedCostControl{
		model:     model,
		costLimit: restr.CostLimit,
	}, nil
}

// GetPaymentControl returns an ImputedCostControl for the payment shard.
// The optional cost limit will be reduced by the current usage with HTLCs.
func (m *ImputedCostManager) GetPaymentControl(paymentHash lntypes.Hash,
	shardAmt lnwire.MilliSatoshi, restrict *ImputedCostRestriction) (
	*ImputedCostControl, error) {

	if restrict == nil {
		return nil, nil
	}

	// With each call of this function, we create a new
	// ImputedCostControl for the specified namespace. Because the
	// imputed costs of all HTLCs are recalculated, this allows a
	// more accurate limit management.
	model, err := m.getNamespacedModel(restrict.Namespace)
	if err != nil {
		return nil, err
	}

	// Unwrap the cost limit, if it exists. If not we return early,
	// because we do not need to calculate the current usage.
	limitUnwr, err := restrict.CostLimit.UnwrapOrErr(errors.New(""))
	if err != nil {
		return &ImputedCostControl{model: model}, nil
	}

	// We calculate the limit for the next payment shard7.
	limit, err := m.calcShardLimit(paymentHash, shardAmt, limitUnwr, model)
	if err != nil {
		return nil, err
	}

	return &ImputedCostControl{
		model:     model,
		costLimit: fn.Some(limit),
	}, nil
}

// calcShardLimit calculates the cost limit in millisatoshis for a payment
// shard.
func (m *ImputedCostManager) calcShardLimit(paymentHash lntypes.Hash, shardAmt,
	costLimit lnwire.MilliSatoshi, model imputedCostModel) (
	lnwire.MilliSatoshi, error) {

	p, err := m.fetchPayment(paymentHash)
	if err != nil {
		return 0, err
	}

	remainAmt := p.GetState().RemainingAmt
	attempts := p.GetHTLCs()

	// cumulative costs over all non-failed attempts.
	var totalCost lnwire.MilliSatoshi

	for _, attempt := range attempts {
		// If the attempt has failed, we don't count its costs.
		if attempt.Failure != nil {
			continue
		}

		// We have to account for the total fees of the route.
		totalCost += attempt.Route.TotalFees()

		// The fromNode of the first hop is the self node.
		fromNode := m.selfNode

		for _, hop := range attempt.Route.Hops {
			// TODO(feelancer21): Think about the correct amount!!!
			totalCost += model.getCost(
				fromNode, hop.PubKeyBytes, hop.AmtToForward,
			)
			fromNode = hop.PubKeyBytes
		}
	}

	// If the state of the namespace has been recently updated, the limit
	// utilization may be higher than the limit.
	if totalCost > costLimit {
		return 0, errInsufficientCostLimit
	}

	// Sanity check that we not exceed the cost limit.
	if shardAmt > remainAmt {
		return costLimit - totalCost, nil
	}

	return shardAmt * (costLimit - totalCost) / remainAmt, nil
}

func (m *ImputedCostManager) QueryNamespaces(names fn.Set[string],
	cb func(name string, ns *ImputedCostNamespace)) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	if names.IsEmpty() {
		names = fn.KeySet(m.namespaces)
	}

	for _, name := range names.ToSlice() {
		ns, err := m.getNamespace(name)
		if err != nil {
			return err
		}

		cb(name, ns.Copy())
	}

	return nil
}

func (m *ImputedCostManager) ImportNamespaces(names fn.Set[string], reset bool,
	cb func(name string, ns *ImputedCostNamespace) error) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	newNamespaces := make(map[string]*ImputedCostNamespace)

	for name, ns := range m.namespaces {
		if !names.Contains(name) {
			newNamespaces[name] = ns
		}
	}

	for _, name := range names.ToSlice() {
		var importedNs *ImputedCostNamespace

		ns, err := m.getNamespace(name)
		if err != nil || reset {
			importedNs = newNamespaceEmpty()
		} else {
			importedNs = ns.Copy()
		}

		err = cb(name, importedNs)
		if err != nil {
			return err
		}

		if importedNs == nil {
			return fmt.Errorf("got nil for namespace %q", name)
		}

		newNamespaces[name] = importedNs
	}

	m.namespaces = newNamespaces

	return nil
}

func (m *ImputedCostManager) DeleteNamespaces(names fn.Set[string]) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, name := range names.ToSlice() {
		if _, err := m.getNamespace(name); err != nil {
			return err
		}

		delete(m.namespaces, name)
	}

	return nil
}
