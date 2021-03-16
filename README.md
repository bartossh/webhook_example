# GO webhook example

## Webhooks

Short explanation [Webhook](https://en.wikipedia.org/wiki/Webhook)

## Dependencies

This example uses go modules to instal dependencies use `go get .` command

- Go version 1.16 or higher
- gorilla mux router
- cors to allow cross origin resource sharing

## Testing

- run server `go run .`
- run script `./test.sh`

## Description

Script `test.sh` runs curl command to the `localhost:8080/webhook` sending json body:

```json
{
  "address": "http://8080/subscriber"
}
```

Server when receiving assigns address from json to the queue chanel of buffer 100.
Than faked computation are performed after which address and payload are send to responder pipeline through chanel of buffer 100
Responder is sending POST request to provided address with calculated payload
