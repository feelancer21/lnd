# Release Notes
- [Bug Fixes](#bug-fixes)
- [New Features](#new-features)
    - [Functional Enhancements](#functional-enhancements)
    - [RPC Additions](#rpc-additions)
    - [lncli Additions](#lncli-additions)
- [Improvements](#improvements)
    - [Functional Updates](#functional-updates)
    - [RPC Updates](#rpc-updates)
    - [lncli Updates](#lncli-updates)
    - [Breaking Changes](#breaking-changes)
    - [Performance Improvements](#performance-improvements)
- [Technical and Architectural Updates](#technical-and-architectural-updates)
    - [BOLT Spec Updates](#bolt-spec-updates)
    - [Testing](#testing)
    - [Database](#database)
    - [Code Health](#code-health)
    - [Tooling and Documentation](#tooling-and-documentation)

# Bug Fixes

* [Fixed a bug](https://github.com/lightningnetwork/lnd/pull/8862) in error
  matching from publishing transactions that can cause the broadcast process to
  fail if `btcd` with an older version (pre-`v0.24.2`) is used.

# New Features
## Functional Enhancements
## RPC Additions
## lncli Additions

<<<<<<< HEAD
=======
* [Added](https://github.com/lightningnetwork/lnd/pull/8491) the `cltv_expiry`
  argument to `addinvoice` and `addholdinvoice`, allowing users to set the
  `min_final_cltv_expiry_delta`

* The [`lncli wallet estimatefeerate`](https://github.com/lightningnetwork/lnd/pull/8730)
  command returns the fee rate estimate for on-chain transactions in sat/kw and
  sat/vb to achieve a given confirmation target.

>>>>>>> fc90bc9b0 (lncli: new command `wallet estimatefeerate`)
# Improvements
## Functional Updates
## RPC Updates
## lncli Updates
## Code Health
## Breaking Changes
## Performance Improvements

# Technical and Architectural Updates
## BOLT Spec Updates

## Testing
## Database
## Code Health
## Tooling and Documentation

# Contributors (Alphabetical Order)

* Yyforyongyu
