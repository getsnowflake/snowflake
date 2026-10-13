# API documentation

Run from the repository root:

```sh
make openapi
make docs-check
```

Requires Go, Swag, Maven, .NET 8 or newer, and Bun 1.4 or newer. Override
Maven's executable with `make openapi MVN=/path/to/mvn` if needed.

`make openapi` regenerates all four service contracts, converts Go's Swagger
documents locally, merges them, enriches documentation metadata, and validates
the result with the pinned Scalar CLI. Carbon exports through its test context
with H2 and RabbitMQ listeners disabled. Oxygen's export mode builds the host
without starting the HTTP server. Neither exporter needs running infrastructure.

The generated `docs/openapi.json` is the file to upload to Scalar. It is
gitignored and should never be edited manually. This command does not update
the externally published copy.

Keep domain descriptions, stable operation IDs and field examples in the source
annotations. The enrichment step declares operation tags, supplies descriptions
for request bodies and components, derives missing media and schema examples
from schemas, and distinguishes reused descriptions with their operation or
schema context. Existing IDs, examples, field names and validation constraints
are preserved. Per-service authentication requirements are carried onto each
operation before merging, so public endpoints remain public.

Generation fails on Scalar errors, warnings and informational findings, except
`camel-case-properties`. That rule is naming advice for the established
snake_case wire format. Renaming only the documentation would produce an
incorrect contract; changing the API fields requires a separate compatibility
decision. Scalar's default online rules still show those naming advisories.

The merged artifact is replaced only after validation passes. TypeScript scripts
run directly with Bun; `make docs-check` runs type checking and Bun tests.
Dependencies are pinned in `bun.lock`. The postinstall script uses Bun to install
Scalar's native Vacuum linter, since Bun skips dependency lifecycle scripts.
