# fitbit-exporter

## Basic usage
This exporter uses some env vars to initialize the oauth client:

```bash
export OAUTH2_CLIENT_ID=client-id
export OAUTH2_CLIENT_SECRET=client-secret
export OAUTH2_REDIRECT_URL=http://localhost:3000/oauth-redirect
export OAUTH2_TOKEN_FILE=token.json # this enables you to persist the token to reuse it even after program exits
```

If there isn't already a token you need to go to `http://localhost:3000/auth` or just directly to `/` which will redirect you if not yet authorized.

### Dev setup

In order to use hot reloading this project uses https://github.com/markbates/refresh. Just run `go get github.com/markbates/refresh` and afterwards you can run this project by just typing `refresh` with hot reloading.

## Metrics

Currently this tool uses the prometheus client library to expose basic metrics. In addition the HTTP client and the rate limiter used to query fitbit data are instrumented and will expose metrics prefixed with `fitbit_`.

## Rate limiting

Fitbit has a rate limit on its API. The implementation leverages a rate limiter within the HTTP transport to make sure it is never exhausted. The client returned from the oauth package will use this limiter if it is set within the `Config` struct.
