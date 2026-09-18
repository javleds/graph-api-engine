# Verification and specification traceability

The normative acceptance cases are in [the v1.1 specification, section 26](specs/spec-api-execution-engine-go-v1.1.md#section-26). Tests use local HTTP servers, temporary SQLite databases, explicit loopback egress exceptions and generated test keys. No provider credentials or external APIs are required.

## Commands

```sh
make check
make coverage fuzz
go test -v ./test/integration
```

The coverage gate requires at least 85% in each pure domain/application package: definition domain measured 99.0%, definition application 97.1%, and execution domain 96.7%. `make coverage` includes mapper and worker callers when measuring these packages.

Fuzz targets: `FuzzDuration`, `FuzzSelector`, `FuzzPathBinding` in `internal/definition/domain`; `FuzzDecodeJSON` in `internal/definition/infrastructure/schema`; `FuzzRequestURLBoundary` in `internal/fetch/application`. Run each separately, for example:

```sh
go test ./internal/definition/domain -run '^$' -fuzz '^FuzzDuration$' -fuzztime=10s -parallel=2
```

All five targets passed bounded one-second runs during implementation; longer scheduled runs are appropriate for continued assurance.

## JSON, composition and pagination

| Spec cases | Behavioral evidence |
|---|---|
| C01–C05 | Mapper `TestNormativeInputExamplesCompile`, `TestNullAndOmittedSelectHaveIdenticalCompilation`; schema `TestSchemaContract`, `TestStrictJSON`. |
| C06–C07 | Domain `TestSelectorDistinguishesMissingNullAndNumericObjectKey`. |
| C08–C09 | Worker `TestNodeTypeMismatchDoesNotCoerceValues`. |
| C10–C12 | Worker `TestCompositionKeepsPositionsAndFreezesAncestorContext`, `TestChildlessScalarsRemainUnwrapped`. |
| C13 | Domain template precision test and full HTTP `TestPartialExecutionCacheIdempotencyAndWebhookRestart` using ID 9007199254740993. |
| C14 | Worker `TestChildrenCompleteInReverseWithoutReorderingCollection` forces the second child to finish first. |
| C15–C17 | Domain `TestTemplateBindingPreservesPrecisionAndEscapes`; mapper semantic rejection tests; fetch URL preservation/traversal tests and fuzzing. |
| C18–C19 | Mapper `TestHeaderAuthCollision` preserves the literal `${SECRET}`. |
| P01–P04, P06–P07 | Worker `TestPaginationSizeAndStartMatrix`, with the exact specified page sizes and start-zero case. |
| P05 | Worker `TestPaginationFiniteCountIsCleanCompletion`. |
| P08 | Worker `TestPaginationLargerPageDiscarded`. |
| P09 | Full HTTP `TestPaginationRetryDoesNotDuplicateAcceptedPages`: page two returns 503 then succeeds; each item appears once. |
| P10–P12 | Worker selector failure, repeated-page and operational-guard tests preserve accepted prefixes. |
| P13 | Worker `TestPaginationChildrenHaveIndependentPageCounters` checks each parent's requested page URLs. |

## Fetching, persistence and delivery

| Spec cases | Behavioral evidence |
|---|---|
| T01–T03, T14–T15 | Fetch quota-scope resolver tests and local admission joint-pacing/conflict tests; cache identity tests assert owner isolation independently of shared quota identity. |
| T04 | Cache service bypass/freshness tests; integration second execution performs no repeated root request. |
| T05 | Local `TestTwentySubscribersConsumeOnePhysicalAdmission` asserts one physical request and admission for twenty consumers. |
| T06–T07, T12, T17 | Local admission tests cover blocked endpoints without early charging, maximum concurrency, cancellation, controlled-clock pacing and cold start. |
| T08–T10, T16 | Retry-After parsing/deadline tests, cooldown SQLite reopen test and gate cooldown behavior. |
| T11, T13 | Fetch retry classes and panic supervision tests; per-attempt hooks and gate tests. |
| T19 | Local `TestBackoffReleasesGlobalSlotForIndependentProvider` advances the independent provider while the other waits in retry backoff. |
| T18 | Worker `TestOneBranchSlotDoesNotDeadlockNestedChildren`. |
| §26.5 | SQLite cache contract tests cover freshness, format, bounds, replacement and isolation; caching service tests cover degradation/HTTP policy; local flight tests cover subscriber cancellation, generation lifetime, response copies and capacity. |
| I01, I03–I05, I18 | Integration acceptance tests exercise client disconnect, concurrent idempotency, owner isolation and low-disk admission; dispatcher reservation tests cover saturation. |
| I02 | Integration `TestCrashRecoveryDoesNotReexecuteAcceptedTrees` exits a subprocess without cleanup after durable acceptance/claim; both pending and running jobs become `PROCESS_INTERRUPTED`, with zero upstream calls. |
| I06, E29 | SQLite atomic-finalization tests and lifecycle retry/exhaustion tests; storage validates completion consistency before transaction commit. |
| I08 | HTTP `TestLostAcknowledgementRetriesDurableEventWithFreshSignature` drops the connection after the receiver accepts the body, then recovers the durable event and verifies retry identity and signature. |
| I07, I09–I10 | Integration webhook retry/restart checks immutable body/event identity and exact HMAC; HTTP sender test uses a controlled clock to verify refreshed timestamps/signatures. |
| I11–I12 | Webhook payload degradation, dispatcher retry/deadline, failed-delivery and outbox tests; notification state is stored separately from execution completion. |
| I13–I14 | Configuration unsupported-driver test; SQLite instance lock/version tests. |
| I15–I16 | Execution store encryption/owner tests and atomic-finalization expiry test. |
| I17 | Egress mixed DNS/private mapped-IP tests and webhook blocked-destination test. |
| I19 | SQLite durability/migration/reconnection test verifies real pragmas and library version. |

## Partial-result matrix

| Spec cases | Behavioral evidence |
|---|---|
| E01–E04, E30, E37 | Worker sibling/descendant failure tests inspect exact fake-fetch call counts; integration partial result checks HTTP 404 failure. |
| E05, E10, E12 | Integration `TestRootAuthorizationExhaustedRetriesAndAllChildrenFailure`: single 401, all children failing with valid root, exactly four 503 attempts and one exhausted diagnostic. |
| E06–E09, E39 | Worker all-roots-failed, available empty-root, null/empty-own-data tests distinguish valid emptiness from a failure placeholder. |
| E13 | Fetch retry/cooldown tests and local physical-admission tests verify retry waits and release; `TestRetriesExhaustExactlyFourPhysicalAdmissions` accounts every physical attempt. |
| E11, E14 | Fetch recovered retry and Retry-After deadline tests; integration pagination retry finishes cleanly. |
| E15–E18 | Worker retained-prefix, selector failure, child-failure pagination and binding-failure tests inspect no-next-page/no-missing-binding requests. |
| E19–E21 | Worker per-consumer failure location and separate response decoding; local flight cancellation/copy tests. `TestSharedFlightTransfersAttemptBudgetAndRetryAccounting` verifies that cancellation of the initiating execution transfers continued physical-work accounting to a surviving consumer. |
| E22–E23 | Worker composition test checks collision/null/nested-array placeholders and skipped-child counts. |
| E24–E25 | Bounded failure collector tests assert numeric deterministic ordering, exact totals, bounded details and continued work. |
| E26–E28 | Worker global-deadline tests retain valid data when publishable and suppress results after technical panic. |
| E31 | Cache degradation test and worker own-data success remain separate from infrastructure telemetry. |
| E32–E33 | Mapper duplicate-root/reference validation and original object/array pointer tests. |
| E34–E35 | Integration partial webhook plus payload-builder tests that omit large results/details explicitly. |
| E36 | Worker `TestRootNamesErrorsAndStatusRemainInsideData`. |
| E38 | Worker `TestEnvelopeHardLimitDoesNotPublishTruncatedJSON`. |
| E40 | Integration partial terminal commit, retryable webhook, process reopen and delivery preserves result/event and does not execute the tree again. |

## Bounded load and measurement limits

`TestBoundedFanoutLoadKeepsStatusResponsive` runs 100 child requests with eight branch slots and four HTTP slots. It checks the physical maximum, a status read while upstream work is deliberately blocked, exact cold-attempt count and zero additional HTTP attempts on a hot execution. It reports heap allocation, live goroutines, WAL bytes and status latency with `-v`.

These are reproducible local correctness/load checks, not production capacity claims. Heap and WAL figures are observations rather than portable fixed thresholds. Profile sustained workloads and real payload distributions before selecting deployment memory and retention budgets.
