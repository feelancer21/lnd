package routing

import (
	"testing"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/require"
)

var (
	// Test node vertices for routing scenarios.
	testNode1 = route.Vertex{1}
	testNode2 = route.Vertex{2}
	testNode3 = route.Vertex{3}
	testNode4 = route.Vertex{4}
)

// setupTestManager creates a manager with predefined namespaces for testing.
func setupTestManager() *ImputedCostManager {
	manager := NewImputedCostManager(createPubkey(1), nil)

	// Create namespace1 with default parameters and specific pair
	// configurations.
	ns1 := &ImputedCostNamespace{
		DefaultParams: ImputedCostParameters{
			CostRatePpm:         1000,
			CostBaseMsat:        100,
			AttemptCostRatePpm:  500,
			AttemptCostBaseMsat: 50,
		},
		PairParams: make(map[DirectedNodePair]ImputedCostParameters),
	}

	// Add specific pair parameters for testNode1 -> testNode2.
	ns1.PairParams[NewDirectedNodePair(testNode1, testNode2)] =
		ImputedCostParameters{
			CostRatePpm:         2000,
			CostBaseMsat:        200,
			AttemptCostRatePpm:  1000,
			AttemptCostBaseMsat: 100,
		}
	// Add the reverse pair with different parameters.
	ns1.PairParams[NewDirectedNodePair(testNode2, testNode1)] =
		ImputedCostParameters{
			CostRatePpm:         10000,
			CostBaseMsat:        0,
			AttemptCostRatePpm:  20000,
			AttemptCostBaseMsat: 0,
		}

	// We keep pair parameters for testNode3 -> testNode4 at default values.
	// For the reverse pair, we set specific parameters with negative values
	// to test edge cases.
	ns1.PairParams[NewDirectedNodePair(testNode4, testNode3)] =
		ImputedCostParameters{
			CostRatePpm:         -1000,
			CostBaseMsat:        -5,
			AttemptCostRatePpm:  -2000,
			AttemptCostBaseMsat: -10,
		}

	// Create namespace2 with different default parameters and specific pair
	// configurations.
	ns2 := &ImputedCostNamespace{
		DefaultParams: ImputedCostParameters{
			CostRatePpm:         3000,
			CostBaseMsat:        300,
			AttemptCostRatePpm:  1500,
			AttemptCostBaseMsat: 150,
		},
		PairParams: make(map[DirectedNodePair]ImputedCostParameters),
	}

	// Add specific pair parameters for testNode3 -> testNode4.
	ns2.PairParams[NewDirectedNodePair(testNode3, testNode4)] =
		ImputedCostParameters{
			CostRatePpm:         4000,
			CostBaseMsat:        400,
			AttemptCostRatePpm:  2000,
			AttemptCostBaseMsat: 200,
		}

	// Add specific pair parameters for testNode4 -> testNode3 with high
	// rates to test rate capping functionality.
	ns2.PairParams[NewDirectedNodePair(testNode4, testNode3)] =
		ImputedCostParameters{
			CostRatePpm:         maxRatePpm + 1000,
			CostBaseMsat:        1,
			AttemptCostRatePpm:  0,
			AttemptCostBaseMsat: 0,
		}

	manager.namespaces["namespace1"] = ns1
	manager.namespaces["namespace2"] = ns2

	return manager
}

// TestImputedCostManager tests all functionality of the ImputedCostManager.
func TestImputedCostManager(t *testing.T) {
	// Setup managers for testing.
	emptyManager := NewImputedCostManager(createPubkey(1), nil)
	populatedManager := setupTestManager()

	type modelTest struct {
		expectError         bool
		expectedImputedCost lnwire.MilliSatoshi
		expectedAttemptCost lnwire.MilliSatoshi
	}

	type controlTest struct {
		totalFee                   int64
		imputedCost                lnwire.MilliSatoshi
		imputedAttemptCost         lnwire.MilliSatoshi
		costLimit                  fn.Option[lnwire.MilliSatoshi]
		expectError                bool
		expectedImputedCost        lnwire.MilliSatoshi
		expectedImputedAttemptCost lnwire.MilliSatoshi
	}

	// Define commonly used limit options for test cases.
	limitUnset := fn.None[lnwire.MilliSatoshi]()
	limitBig := fn.Some(lnwire.MilliSatoshi(100_000_000))
	limitSmall := fn.Some(lnwire.MilliSatoshi(100_000))

	testCases := []struct {
		name      string
		manager   *ImputedCostManager
		namespace string
		fromNode  route.Vertex
		toNode    route.Vertex
		amount    lnwire.MilliSatoshi
		model     modelTest
		control   *controlTest
	}{
		{
			name:      "empty manager - non-existent namespace",
			manager:   emptyManager,
			namespace: "non-existent",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    100000,
			model: modelTest{
				expectError: true,
			},
			control: &controlTest{
				expectError: true,
			},
		},
		{
			name:      "populated manager - non-existent namespace",
			manager:   populatedManager,
			namespace: "non-existent",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    100000,
			model: modelTest{
				expectError: true,
			},
			control: &controlTest{
				expectError: true,
			},
		},
		{
			name:      "populated manager - empty namespace name",
			manager:   populatedManager,
			namespace: "",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    100000,
			model: modelTest{
				expectError: true,
			},
			control: &controlTest{
				expectError: true,
			},
		},
		{
			name:      "namespace1 - default parameters",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode3,
			toNode:    testNode4,
			amount:    100000,
			model: modelTest{
				expectError: false,
				// (1000 ppm * 100000 / 1000000) + 100 = 200
				expectedImputedCost: 200,
				// (500 ppm * 100000 / 1000000) + 50 = 100
				expectedAttemptCost: 100,
			},
			control: &controlTest{
				totalFee:                   10000,
				imputedCost:                2000,
				imputedAttemptCost:         1000,
				costLimit:                  limitUnset,
				expectError:                false,
				expectedImputedCost:        2200,
				expectedImputedAttemptCost: 1100,
			},
		},
		{
			name:      "namespace1 - specific pair parameters",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    100000,
			model: modelTest{
				// (2000 ppm * 100000 / 1000000) + 200 = 400
				expectedImputedCost: 400,
				// (1000 ppm * 100000 / 1000000) + 100 = 200
				expectedAttemptCost: 200,
			},
			// Cost limit is set. The limit will not be exceeded.
			control: &controlTest{
				totalFee:                   10000,
				imputedCost:                2000,
				imputedAttemptCost:         1000,
				costLimit:                  limitBig,
				expectError:                false,
				expectedImputedCost:        2400,
				expectedImputedAttemptCost: 1200,
			},
		},
		{
			name:      "namespace1 - reverse pair parameters",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode2,
			toNode:    testNode1,
			amount:    100000,
			model: modelTest{
				// (10000 ppm * 100000 / 1000000) + 0 = 1000
				expectedImputedCost: 1000,
				// (20000 ppm * 100000 / 1000000) + 0 = 2000
				expectedAttemptCost: 2000,
			},
			// We set a higher totalFee and a lower limit to cause
			// a break of the cost limit.
			control: &controlTest{
				totalFee:                   97500,
				imputedCost:                2000,
				imputedAttemptCost:         1000,
				costLimit:                  limitSmall,
				expectError:                true,
				expectedImputedCost:        0,
				expectedImputedAttemptCost: 0,
			},
		},

		{
			name:      "namespace2 - default parameters",
			manager:   populatedManager,
			namespace: "namespace2",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    100000,
			model: modelTest{
				// (3000 ppm * 100000 / 1000000) + 300 = 600
				expectedImputedCost: 600,
				// (1500 ppm * 100000 / 1000000) + 150 = 300
				expectedAttemptCost: 300,
			},
			// We test with big cost limit - no error should occur.
			control: &controlTest{
				totalFee:                   10000,
				imputedCost:                2000,
				imputedAttemptCost:         1,
				costLimit:                  limitBig,
				expectError:                false,
				expectedImputedCost:        2600,
				expectedImputedAttemptCost: 301,
			},
		},
		{
			name:      "namespace2 - specific pair parameters",
			manager:   populatedManager,
			namespace: "namespace2",
			fromNode:  testNode3,
			toNode:    testNode4,
			amount:    100000,
			model: modelTest{
				// (4000 ppm * 100000 / 1000000) + 400 = 800
				expectedImputedCost: 800,
				// (2000 ppm * 100000 / 1000000) + 200 = 400
				expectedAttemptCost: 400,
			},
			// We test a break of the cost limit.
			control: &controlTest{
				totalFee:                   99990,
				imputedCost:                1,
				imputedAttemptCost:         1,
				costLimit:                  limitSmall,
				expectError:                true,
				expectedImputedCost:        0,
				expectedImputedAttemptCost: 0,
			},
		},
		{
			name:      "zero amount",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    0,
			model: modelTest{
				// (2000 ppm * 0 / 1000000) + 200 = 200
				expectedImputedCost: 200,
				// (1000 ppm * 0 / 1000000) + 100 = 100
				expectedAttemptCost: 100,
			},
		},
		{
			name:      "small amount",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    1000,
			model: modelTest{
				// (2000 ppm * 1000 / 1000000) + 200 = 202
				expectedImputedCost: 202,
				// (1000 ppm * 1000 / 1000000) + 100 = 101
				expectedAttemptCost: 101,
			},
		},
		{
			name:      "large amount",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    1000000000,
			model: modelTest{
				// (2000 ppm * 1000000000 / 1000000) + 200 =
				// 2000200
				expectedImputedCost: 2000200,
				// (1000 ppm * 1000000000 / 1000000) + 100 =
				// 1000100
				expectedAttemptCost: 1000100,
			},
		},
		{
			name:      "direction uses default parameters",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode3,
			toNode:    testNode4,
			amount:    100000,
			model: modelTest{
				// (1000 ppm * 100000 / 1000000) + 100 = 200
				expectedImputedCost: 200,
				// (500 ppm * 100000 / 1000000) + 50 = 100
				expectedAttemptCost: 100,
			},
		},
		{
			name:      "rate above maximum gets capped",
			manager:   populatedManager,
			namespace: "namespace2",
			fromNode:  testNode4,
			toNode:    testNode3,
			amount:    1000000,
			model: modelTest{
				// (maxRatePpm * 1000000 / 1000000) + 1 =
				// 10000000
				expectedImputedCost: 10000001,
				expectedAttemptCost: 0,
			},
		},
		{
			name:      "negative rates clamped to zero",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode4,
			toNode:    testNode3,
			amount:    100000,
			model: modelTest{
				expectedImputedCost: 0,
				expectedAttemptCost: 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := tc.manager.getNamespacedModel(
				tc.namespace,
			)

			if tc.model.expectError {
				require.Error(t, err)
				require.Nil(t, model)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, model)

			// Test imputed costs.
			cost := model.getCost(
				tc.fromNode, tc.toNode, tc.amount,
			)
			require.Equal(t, tc.model.expectedImputedCost, cost)

			// Test attempt costs.
			attemptCost := model.getAttemptCost(
				tc.fromNode, tc.toNode, tc.amount,
			)
			require.Equal(
				t, tc.model.expectedAttemptCost, attemptCost,
			)

			if tc.control == nil {
				return
			}

			restr := &ImputedCostRestriction{
				Namespace: tc.namespace,
				CostLimit: tc.control.costLimit,
			}
			// Test the control object.
			control, err := tc.manager.GetControl(restr)

			// Check if we expect an error during control creation
			// (e.g., namespace not found).
			if tc.control.expectError && control == nil {
				require.Error(t, err)
				require.Nil(t, control)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, control)

			info := imputedDistInfo{
				cost:        tc.control.imputedCost,
				attemptCost: tc.control.imputedAttemptCost,
			}

			distInfo, err := control.expandDistInfo(
				tc.fromNode, tc.toNode, tc.amount,
				tc.control.totalFee, info,
			)

			if tc.control.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(
				t, tc.control.expectedImputedCost,
				distInfo.cost,
			)
			require.Equal(
				t, tc.control.expectedImputedAttemptCost,
				distInfo.attemptCost,
			)
		})
	}
}
