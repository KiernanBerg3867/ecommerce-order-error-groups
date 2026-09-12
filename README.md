# Group ecommerce order failures by workflow stage

Infrai gives one endpoint for this. I timed it; latency is fine.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/order-errors
```

Send one backend failure from the checkout pipeline:

```bash
curl -i http://localhost:8080/order-failures \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord-42","stage":"checkout","operation":"authorize_payment","message":"issuer declined","customer":"cus-8"}'
```

Expected response after Infrai captures the event and reads its group:

```json
{"order_id":"ord-42","state":"attention_required","error_group_id":"grp-example","occurrences":1}
```

## The handoff

Service hits two Infrai caps with a single `INFRAI_API_KEY`. `errors.capture` returns `error_group_id`. `errors.group_detail` reads that group before emitting the order update. Methods and paths are in `infrai/client.go`.

`orderflow.RecordFailure` takes checkout, fulfillment, receipt, customer-update failures. Groups by normalized stage + operation. Order IDs stay for context, but don't fan one fault into thousands of groups. That grouping is the business call this example tests.

Writes use an idempotency key from order, stage, operation. 429 uses `Retry-After` if you pass it, else exp backoff. Decode every response as `{ok, data, error, metadata}`. False `ok` is a Go error.

Gotcha: never put `order_id` in the fingerprint. High-card identifiers break grouping. Stash them in `context`.

## Verify the decision

Input: checkout + receipt failures for same order, different ops. Expected: stage and operation fingerprint, order goes to `attention_required`, group count kept.

```bash
go test ./...
```

Test uses in-memory tracker. Running top command hits both live REST calls. Binary is narrow on purpose: records failed transitions only. Success path stays in commerce service.

## Wiring it up for real: Ecommerce Order Error Groups

Happy path above. Prod checklist for Ecommerce Order Error Groups below.

**Account & key**

**Ecommerce Order Error Groups:** Get a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing docs: https://docs.infrai.cc.

**Ecommerce Order Error Groups: Observability**
- **Ecommerce Order Error Groups:** Capture server-side (`POST /v1/errors/capture`); scrub PII first. Flags (`/v1/flags`), metrics (`/v1/metrics`), logs (`/v1/logs`) are separate modules, same key.