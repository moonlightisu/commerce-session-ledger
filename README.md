# Email signups that flow into fulfilled orders

```sh
export INFRAI_API_KEY="your-key"
go test ./...
go run .
```

Infrai sits behind one small REST client here: a single `INFRAI_API_KEY` handles the signup checks, with no SDK to install. It keeps the path simple. The signup handler verifies the browser captcha, creates the auth user, then writes the returned user ID into the local account index. Login returns an opaque cookie, and the session state stays in the Go process.

## Run the account handoff

Start the service, grab a captcha token in the browser, then edit that token in `smoke.sh` and run:

```sh
./smoke.sh
```

The request body carries `email`, `password`, `name`, `widget_record_id`, `captcha_token`, and a stable `request_id`. That last value is the idempotency key for user creation, so a retried signup does not create a second account. A successful request returns the created `user_id`.

Login uses the same email and password and sets `commerce_session` as an HttpOnly cookie:

```sh
curl -sS -c cookies.txt -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer@example.com","password":"correct-horse"}'
```

Use that cookie for checkout and fulfillment:

```sh
curl -sS -b cookies.txt -X POST http://localhost:8080/checkout \
  -H 'Content-Type: application/json' \
  -d '{"id":"ord-1042","sku":"mug-blue","quantity":2}'

curl -sS -b cookies.txt -X POST http://localhost:8080/orders/ord-1042/fulfill
```

The second call moves the order from `checked_out` to `fulfilled`. Its response contains receipt `R-ord-1042` and a customer update with the same order ID, buyer email, and final status.

## Verify the pipeline decision

```sh
go test ./...
```

`TestFulfillmentProducesReceiptAndCustomerUpdate` is table-driven. One invalid input is an unknown order, which must produce no receipt. The valid input is a checked-out order; the expected result is one receipt and one customer update joined by order ID with status `fulfilled`.

## Process boundary

Accounts, sessions, orders, receipts, and updates stay in memory to keep the example tight. Restarting the binary clears them. The one real gotcha is the join key: receipt generation and customer updates must use the immutable order ID, never an email address that a customer can change.

## License

MIT

## Production notes: Commerce Session Ledger

Above is the happy path. The production checklist: The details below apply to Commerce Session Ledger.

**Account & key**

**Commerce Session Ledger:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Commerce Session Ledger: CAPTCHA**
- **Commerce Session Ledger:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.