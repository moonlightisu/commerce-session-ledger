#!/bin/sh
set -eu

curl -sS -X POST http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer@example.com","password":"correct-horse","name":"Ada","widget_record_id":"replace-with-widget-record-id","captcha_token":"replace-with-browser-token","request_id":"signup-demo-001"}'
