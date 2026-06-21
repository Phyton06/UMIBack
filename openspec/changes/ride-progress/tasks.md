# Tasks: ride-progress

## Phase 1: Handlers

- [x] 1.1 Add `EnRouteRide` handler — ACCEPTED→EN_ROUTE
- [x] 1.2 Add `ArrivedRide` handler — EN_ROUTE→ARRIVED
- [x] 1.3 Add `StartRide` handler — ARRIVED→IN_PROGRESS
- [x] 1.4 Add `CompleteRide` handler — IN_PROGRESS→COMPLETED (+ completed_at)

## Phase 2: Routing

- [x] 2.1 Register 4 routes in main.go (all RequireRole("driver"))

## Phase 3: Tests

- [x] 3.1 Write tests for en-route (happy, wrong status, not assigned, not found)
- [x] 3.2 Write tests for arrived (happy, wrong status, not assigned)
- [x] 3.3 Write tests for start (happy, wrong status, not assigned)
- [x] 3.4 Write tests for complete (happy, wrong status, not assigned, checks completed_at)
- [x] 3.5 Write sequential E2E test: accept→en-route→arrived→start→complete

## Review Workload Forecast

- 400-line budget risk: Low
- Chained PRs recommended: No
- Decision needed before apply: No
