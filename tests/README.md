### Rate limiting in integration & E2E tests

Production per-endpoint rate limits help prevent abuse (e.g. credential stuffing, rapid OTP requests). During integration or end-to-end test runs these limits can introduce flakiness or slow execution. A global disable flag is provided specifically for test environments:

Set environment variable before starting the API container:

```
API_DISABLE_RATE_LIMIT=true
```

Effect:
- All `middleware.NewLimit(...)` wrappers become no-ops.
- Session deletion endpoint uses a passthrough instead of the default limiter.
- Log line emitted on startup:

```
Rate limiting disabled by configuration (API_DISABLE_RATE_LIMIT=true)
```

Recommended usage patterns:
1. General integration/E2E test jobs: enable flag for speed and determinism.
2. Dedicated rate limit validation suite: leave flag OFF to assert 429 behavior and boundary conditions.
3. Never enable flag in staging or production deployments.

docker-compose override example snippet:

```
	api:
		environment:
			- API_DISABLE_RATE_LIMIT=true
```

If you need to test multiple profiles concurrently, ensure each test job explicitly sets or unsets the flag to avoid unintended sharing of limiter state.

