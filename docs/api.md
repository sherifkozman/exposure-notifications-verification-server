# API access

Access to the verification server API requires an API key. An API key typically
corresponds to an individual mobile application or individual human. There are
two types of API keys:

-   `DEVICE` - Intended for a mobile application to call the `cmd/apiserver` to
    perform the two step protocol to exchange verification _codes_ for
    verification _tokens_, and verification _tokens_ for verification
    _certificates_.

-   `ADMIN` - Intended for public health authority internal applications to
    integrate with this server. **We strongly advise putting additional
    protections in place such as an external proxy authentication.**


# API usage

The following APIs exist for the API server (`cmd/apiserver`). All APIs are JSON
over HTTP. You should always specify the `content-type` and `accept` headers as
`application/json`. Check with your server operator for the specific hostname.

## Authenticating

All endpoints require an API key passed via the `X-API-Key` header. The server
supports HTTP/2, so the header key is case-insensitive. For example:

```sh
curl https://example.encv.org/api/method \
  --header "content-type: application/json" \
  --header "accept: application/json" \
  --header "x-api-key: abcd.5.dkanddssk"
```

API keys will _generally_ be in a particular format, but developers should not
attempt to build any intelligence on this format. The format, length, and
character set are not guaranteed to remain the same between releases.

## Error reporting

All errors contain an English language error message and well defines `ErrorCode`.
The `ErrorCodes` are defined in [api.go](https://github.com/google/exposure-notifications-verification-server/blob/main/pkg/api/api.go).

# API Methods

## `/api/verify` 

Exchange a verification code for a long term verification token.

**VerifyCodeRequest:**

```json
{
  "code": "<the code>",
  "accept": ["confirmed"]
}
```

* `accept` is an _optional_ list of the diagnosis types that the client is willing to process. Accepted values are
  * `["confirmed"]`
  * `["confirmed", "likely"]` 
  * `["confirmed", "likely", "negative"]`
  * It is not possible to get just `likely` or just `negative` - if a client
        passes `likely` they are indiciating they can process both `confirmed` and `likely`.

**VerifyCodeResponse:**

```json
{
  "TestType": "<test type string>",
  "SymptomDate": "YYYY-MM-DD",
  "VerificationToken": "<JWT verification token>",
  "Error": "",
  "ErrorCode": "",
}
```

Possible error code responses. New error codes may be added in future releases.

| ErrorCode               | HTTP Status | Retry | Meaning |
|-------------------------|-------------|-------|---------|
| `unparsable_request`    | 400         | No    | Client sent an request the sever cannot parse |
| `code_invalid`          | 400         | No    | Code invalid or used, user may need to obtain a new code. |
| `code_expired`          | 400         | No    | Code has expired, user may need to obtain a new code. |
| `code_not_found`        | 400         | No    | The server has no record of that code. |
| `invalid_test_type`     | 400         | No    | The client sent an accept of an unrecgonized test type |
| `unsupported_test_type` | 412         | No    | The code may be valid, but represents a test type the client cannot process. User may need to upgrade software. |
|                         | 500         | Yes   | Internal processing error, may be successful on retry. |

## `/api/certificate`

Exchange a verification token for a verification certificate (for sending to a key server)

**VerificationCertificateRequest:**

```json
{
  "token": "token from verifyCodeResponse",
  "ekeyhmac": "hmac of exposure keys, base64 encoded"
}
```

* `token`: must be exactly the string that was returned on the `/api/verify` request
* `ekeyhmac`: must be calculated on the client
  * The client generates an HMAC secret and calcualtes the HMAC of the actual TEK data
  * [Plaintext generation algorithm](https://github.com/google/exposure-notifications-server/blob/main/docs/design/verification_protocol.md)
  * [Sample HMAC generation (Go)](https://github.com/google/exposure-notifications-server/blob/main/pkg/verification/utils.go)
  * The key server will re-calculate this HMAC and it MUST match what is presented here.


**VerificationCertificateResponse:**

 ```json
{
  "Certificate": "<JWT verification certificate>",
  "Error": "",
  "ErrorCode": ""
}
```

Possible error code responses. New error codes may be added in future releases.

| ErrorCode               | HTTP Status | Retry | Meaning |
|-------------------------|-------------|-------|---------|
| `token_invalid`         | 400         | No    | The provided token is invalid, or already used to generate a certificate |
| `token_expired`         | 400         | No    | Code invalid or used, user may need to obtain a new code. |
| `hmac_invalid`          | 400         | No    | The `ekeyhmac` field, when base64 decoded is not the right size (32 bytes) |
|                         | 500         | Yes   | Internal processing error, may be successful on retry. |

# Chaffing requests

In addition to "real" requests, the server also accepts chaff (fake) requests.
These can be used to obfuscate real traffic from a network observer or server
operator. To initiate a chaff request, set the `X-Chaff` header on your request:

```sh
curl https://example.encv.org/api/endpoint \
  --header "content-type: application/json" \
  --header "accept: application/json" \
  --header "x-chaff: 1"
```

The client should still send a real request with a real request body (the body
will not be processed). The server will respond with a fake response that your
client **MUST NOT** process. Client's should sporadically issue chaff requests.

# Response codes overview

You can expect the following responses from this API:

-   `400` - The client made a bad/invalid request. Search the JSON response body
    for the `"errors"` key. The body may be empty.

-   `401` - The client is unauthorized. This could be an invalid API key or
    revoked permissions. This usually has no `"errors"` key, but clients can try
    to read the JSON body to see if there's additional information (it may be
    empty)

-   `404` - The client made a request to an invalid URL (routing error). Do not
    retry.

-   `405` - The client used the wrong HTTP verb. Do not retry.

-   `412` - The client requested a precondition that cannot be satisfied.

-   `429` - The client is rate limited. Check the `X-Retry-After` header to
    determine when to retry the request. Clients can also monitor the
    `X-RateLimit-Remaining` header that's returned with all responses to
    determine their rate limit and rate limit expiration.

-   `5xx` - Internal server error. Clients should retry with a reasonable
    backoff algorithm and maximum cap.
