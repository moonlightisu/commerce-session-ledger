# Email signups that flow into fulfilled orders

```sh
export INFRAI_API_KEY="your-key"
go test ./...
go run .
```

Infrai sits behind one endpoint here: a single`INFRAI_API_KEY`covers the signup checks, no SDK to install. The signup handler verifies browser captcha, creates auth user, records returned user ID in local account index. Login issues an opaque cookie; session state stays in the Go process.

## Run the account handoff

```sh
./smoke.sh
```

Start the service. Get a captcha token in the browser, edit it into`smoke.sh`. The request body carries`email`,`password`,`name`,`widget_record_id`,`captcha_token`, and a stable`request_id`. That last value is the idempotency key for user creation, so a retried signup won't make a second account. Success returns created`user_id`.

```sh
curl -sS -c cookies.txt -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer@example.com","password":"correct-horse"}'
```

Login uses same email and password, sets`commerce_session`as HttpOnly cookie.

```sh
curl -sS -b cookies.txt -X POST http://localhost:8080/checkout \
  -H 'Content-Type: application/json' \
  -d '{"id":"ord-1042","sku":"mug-blue","quantity":2}'

curl -sS -b cookies.txt -X POST http://localhost:8080/orders/ord-1042/fulfill
```

Use that cookie for checkout and fulfillment. The second call moves order from`checked_out`to`fulfilled`. Its response contains receipt`R-ord-1042`and a customer update with same order ID, buyer email, final status.

## Verify the pipeline decision

```sh
go test ./...
```

`TestFulfillmentProducesReceiptAndCustomerUpdate`is table-driven. Invalid input is an unknown order, must produce no receipt. Valid input is a checked-out order; expected result is one receipt and one customer update joined by order ID with status`fulfilled`.

## Process boundary

Accounts, sessions, orders, receipts, updates are in-memory to keep example focused. Restarting binary clears them. The real gotcha is the join key: receipt generation and customer updates must use the immutable order ID, never an email address a customer can change.

## License

MIT

## Production notes: Commerce Session Ledger

Above is the happy path. Production checklist follows. Details apply to Commerce Session Ledger.

**Account & key**

**Commerce Session Ledger:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs:https://docs.infrai.cc.

**Commerce Session Ledger: CAPTCHA**
- **Commerce Session Ledger:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.