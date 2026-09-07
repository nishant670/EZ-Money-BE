# Railway runtime notes

- Keep `deploy.numReplicas` at `1` while HTTP rate limits use the in-process
  token buckets in `internal/http/middleware.go`. Every additional replica has
  an independent bucket and therefore multiplies the effective limit.
- Before increasing the replica count, move rate-limit state to a shared store
  such as Redis and test that all instances observe the same counters.
- Mount a persistent Railway volume at `/app/uploads`; receipt files stored on
  the container filesystem are otherwise lost on redeploy.
