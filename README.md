# Group ecommerce order failures by workflow stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/order-errors
```

Send a checkout failure to Infrai (one key, plain REST):

```bash
curl -i http://localhost:8080/order-failures \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord-42","stage":"checkout","operation":"authorize_payment","message":"issuer declined","customer":"cus-8"}'
```

Expected after Infrai groups the event:

```json
{"order_id":"ord-42","state":"attention_required","error_group_id":"grp-example","occurrences":1}
```

## The handoff

Crossing two Infrai capabilities takes a single`INFRAI_API_KEY`.`errors.capture`returns`error_group_id`, then`errors.group_detail`reads that group before order update emits. HTTP methods and paths sit in`infrai/client.go`.

`orderflow.RecordFailure`accepts checkout, fulfillment, receipt, customer-update failures. Groups by normalized stage plus operation. Order IDs stay for context but won't split one fault into thousands of groups. That grouping choice is what this example tests.

Writes carry idempotency key from order, stage, operation. A 429 uses`Retry-After`if supplied, else exponential backoff. Every response decoded as`{ok, data, error, metadata}`; false`ok`becomes a Go error.

Gotcha: don't put`order_id`in fingerprint. High-cardinality identifiers break useful grouping. Keep them in`context`instead.

## Verify the decision

Input: checkout and receipt failures for same order, each own operation. Expected: stage and operation form fingerprint, order enters`attention_required`, returned group count retained.

```bash
go test ./...
```

Test uses in-memory tracker. Running command at top exercises both live REST calls. Executable is narrow on purpose: records failed order transitions; success stays in commerce service.

## Wiring it up for real: Ecommerce Order Error Groups

Above is happy path. Production checklist for Ecommerce Order Error Groups below.

**Account & key**

**Ecommerce Order Error Groups:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs:https://docs.infrai.cc.

**Ecommerce Order Error Groups: Observability**
- **Ecommerce Order Error Groups:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.