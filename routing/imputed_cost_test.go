package routing

import (
	"testing"

	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/require"
)

var (
	testNode1 = route.Vertex{1}
	testNode2 = route.Vertex{2}
	testNode3 = route.Vertex{3}
	testNode4 = route.Vertex{4}
)

// setupTestManager creates a manager with predefined namespaces for testing.
func setupTestManager() *ImputedCostManager {
	manager := NewImputedCostManager()

	// Create namespace1 with default params and specific pair.
	ns1 := &imputedCostNamespace{
		defaultParams: imputedCostParameters{
			costRatePpm:         1000,
			costBaseMsat:        100,
			attemptCostRatePpm:  500,
			attemptCostBaseMsat: 50,
		},
		pairParams: make(map[DirectedNodePair]imputedCostParameters),
	}

	// Add specific pair parameters for testNode1 -> testNode2.
	ns1.pairParams[NewDirectedNodePair(testNode1, testNode2)] =
		imputedCostParameters{
			costRatePpm:         2000,
			costBaseMsat:        200,
			attemptCostRatePpm:  1000,
			attemptCostBaseMsat: 100,
		}
	// Add the reverse pair with different parameters.
	ns1.pairParams[NewDirectedNodePair(testNode2, testNode1)] =
		imputedCostParameters{
			costRatePpm:         10000,
			costBaseMsat:        0,
			attemptCostRatePpm:  20000,
			attemptCostBaseMsat: 0,
		}

	// We keep pair parameters for testNode3 -> testNode4 at default values.
	// For the reverse pair, we set specific parameters.
	ns1.pairParams[NewDirectedNodePair(testNode4, testNode3)] =
		imputedCostParameters{
			costRatePpm:         -1000,
			costBaseMsat:        -5,
			attemptCostRatePpm:  -2000,
			attemptCostBaseMsat: -10,
		}

	// Create namespace2 with different default params and specific pair.
	ns2 := &imputedCostNamespace{
		defaultParams: imputedCostParameters{
			costRatePpm:         3000,
			costBaseMsat:        300,
			attemptCostRatePpm:  1500,
			attemptCostBaseMsat: 150,
		},
		pairParams: make(map[DirectedNodePair]imputedCostParameters),
	}

	// Add specific pair parameters for testNode3 -> testNode4.
	ns2.pairParams[NewDirectedNodePair(testNode3, testNode4)] =
		imputedCostParameters{
			costRatePpm:         4000,
			costBaseMsat:        400,
			attemptCostRatePpm:  2000,
			attemptCostBaseMsat: 200,
		}

	// Add specific pair parameters for testNode4 -> testNode3 with high
	// rates.
	ns2.pairParams[NewDirectedNodePair(testNode4, testNode3)] =
		imputedCostParameters{
			costRatePpm:         maxRatePpm + 1000,
			costBaseMsat:        1,
			attemptCostRatePpm:  0,
			attemptCostBaseMsat: 0,
		}

	manager.namespaces["namespace1"] = ns1
	manager.namespaces["namespace2"] = ns2

	return manager
}

// TestImputedCostManager tests all functionality of the ImputedCostManager.
func TestImputedCostManager(t *testing.T) {
	// Setup managers for testing.
	emptyManager := NewImputedCostManager()
	populatedManager := setupTestManager()

	type modelTest struct {
		expectedError       error
		expectedImputedCost lnwire.MilliSatoshi
		expectedAttemptCost lnwire.MilliSatoshi
	}

	type controlTest struct {
		totalFee                   int64
		absoluteAttemptCost        float64
		imputedCost                lnwire.MilliSatoshi
		imputedAttemptCost         lnwire.MilliSatoshi
		costLimit                  fn.Option[lnwire.MilliSatoshi]
		attemptCostLimit           fn.Option[lnwire.MilliSatoshi]
		expectedError              error
		expectedImputedCost        lnwire.MilliSatoshi
		expectedImputedAttemptCost lnwire.MilliSatoshi
	}
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
			name:      "empty manager - non-existent ns",
			manager:   emptyManager,
			namespace: "non-existent",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    100000,
			model: modelTest{
				expectedError: errNamespaceNotFound,
			},
			control: &controlTest{
				expectedError: errNamespaceNotFound,
			},
		},
		{
			name:      "populated manager - non-existent ns",
			manager:   populatedManager,
			namespace: "non-existent",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    100000,
			model: modelTest{
				expectedError: errNamespaceNotFound,
			},
			control: &controlTest{
				expectedError: errNamespaceNotFound,
			},
		},
		{
			name:      "populated manager - empty ns name",
			manager:   populatedManager,
			namespace: "",
			fromNode:  testNode1,
			toNode:    testNode2,
			amount:    100000,
			model: modelTest{
				expectedError: errNamespaceNotFound,
			},
			control: &controlTest{
				expectedError: errNamespaceNotFound,
			},
		},
		{
			name:      "namespace1 - default params",
			manager:   populatedManager,
			namespace: "namespace1",
			fromNode:  testNode3,
			toNode:    testNode4,
			amount:    100000,
			model: modelTest{
				expectedError: nil,
				// (1000 ppm * 100000 / 1000000) + 100 = 200
				expectedImputedCost: 200,
				// (500 ppm * 100000 / 1000000) + 50 = 100
				expectedAttemptCost: 100,
			},
			control: &controlTest{
				totalFee:                   10000,
				absoluteAttemptCost:        10000,
				imputedCost:                2000,
				imputedAttemptCost:         1000,
				costLimit:                  limitUnset,
				attemptCostLimit:           limitUnset,
				expectedError:              nil,
				expectedImputedCost:        2200,
				expectedImputedAttemptCost: 1100,
			},
		},
		{
			name:      "namespace1 - specific pair params",
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
			// cost limit and attempt cost limit are set. Both limits
			// will not be exceeded.
			control: &controlTest{
				totalFee:                   10000,
				absoluteAttemptCost:        10000,
				imputedCost:                2000,
				imputedAttemptCost:         1000,
				costLimit:                  limitBig,
				attemptCostLimit:           limitBig,
				expectedError:              nil,
				expectedImputedCost:        2400,
				expectedImputedAttemptCost: 1200,
			},
		},
		{
			name:      "namespace1 - reverse pair params",
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
			// we set a higher totalFee and a lower limit to cause
			// a break of the cost limit.
			control: &controlTest{
				totalFee:            97500,
				absoluteAttemptCost: 10000,
				imputedCost:         2000,
				imputedAttemptCost:  1000,
				costLimit:           limitSmall,
				attemptCostLimit:    limitBig,
				expectedError:       errInsufficientCostLimit,
				// values keep the same as above, because of the
				// limit break.
				expectedImputedCost:        2000,
				expectedImputedAttemptCost: 1000,
			},
		},

		{
			name:      "namespace2 - default params",
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
			// We test a break of the attempt cost limit now.
			control: &controlTest{
				totalFee:            10000,
				absoluteAttemptCost: 99990,
				imputedCost:         2000,
				imputedAttemptCost:  1,
				costLimit:           limitBig,
				attemptCostLimit:    limitSmall,
				expectedError:       errInsufficientAttemptCostLimit,
				// values keep the same as above, because of the
				// limit break.
				expectedImputedCost:        2000,
				expectedImputedAttemptCost: 1,
			},
		},
		{
			name:      "namespace2 - specific pair params",
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
			// we test a break of both limits now.
			control: &controlTest{
				totalFee:            99990,
				absoluteAttemptCost: 99990,
				imputedCost:         1,
				imputedAttemptCost:  1,
				costLimit:           limitSmall,
				attemptCostLimit:    limitSmall,
				// first returned error is for the cost limit.
				expectedError: errInsufficientCostLimit,
				// values keep the same as above, because of the
				// limit break.
				expectedImputedCost:        1,
				expectedImputedAttemptCost: 1,
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
				// (2000 ppm * 1000000000 / 1000000) + 200 2000200
				expectedImputedCost: 2000200,
				// (1000 ppm * 1000000000 / 1000000) + 100 = 1000100
				expectedAttemptCost: 1000100,
			},
		},
		{
			name:      "direction uses default",
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
				// (maxRatePpm * 1000000 / 1000000) + 1 = 10000000
				expectedImputedCost: 10000001,
				expectedAttemptCost: 0,
			},
		},
		{
			name:      "negative rates",
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

			if tc.model.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tc.model.expectedError, err)
				require.Nil(t, model)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, model)

			// Test imputed cost.
			cost := model.getCost(
				tc.fromNode, tc.toNode, tc.amount,
			)
			require.Equal(t, tc.model.expectedImputedCost, cost)

			// Test attempt cost.
			attemptCost := model.getAttemptCost(
				tc.fromNode, tc.toNode, tc.amount,
			)
			require.Equal(t, tc.model.expectedAttemptCost, attemptCost)

			if tc.control == nil {
				return
			}

			// Test the control object
			control, err := tc.manager.GetNamespacedControl(
				tc.namespace,
				tc.control.costLimit,
				tc.control.attemptCostLimit,
			)

			// Check if we expect an error during control creation (e.g., namespace not found)
			if tc.control.expectedError == errNamespaceNotFound {
				require.Error(t, err)
				require.Equal(t, tc.control.expectedError, err)
				require.Nil(t, control)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, control)

			// Test the processPair method
			imputedCost := tc.control.imputedCost
			imputedAttemptCost := tc.control.imputedAttemptCost

			err = control.processPair(
				tc.fromNode, tc.toNode, tc.amount,
				tc.control.totalFee,
				tc.control.absoluteAttemptCost,
				&imputedCost, &imputedAttemptCost,
			)

			if tc.control.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tc.control.expectedError, err)
				// Values should remain unchanged when there's an error
				require.Equal(t, tc.control.expectedImputedCost, imputedCost)
				require.Equal(t, tc.control.expectedImputedAttemptCost, imputedAttemptCost)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.control.expectedImputedCost, imputedCost)
				require.Equal(t, tc.control.expectedImputedAttemptCost, imputedAttemptCost)
			}

		})
	}
}
