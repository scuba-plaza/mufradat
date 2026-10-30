# Integration tests

These tests call the real Mistral and Google APIs. They are behind the
`integration` build tag, so `go test ./...`, CI and the Nix check never compile
or run them. They pin what the fake servers in the default suite can only
assume: the endpoints, request bodies and reply shapes the code relies on.

## Running them

```sh
MISTRAL_API_KEY="$(cat ~/.config/mufradat/mistral-api-key)" \
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json \
go test -tags integration ./...
```

With the tag on, a missing or invalid credential is a test **failure**, never a
skip. Every test first proves its credential with a call that costs nothing
(Mistral `GET /v1/models`, Google `GET /v1/voices`) and says which variable to
fix. `go vet -tags integration ./...` checks that the tests compile.

To run only one service, name its package:

```sh
MISTRAL_API_KEY=... go test -tags integration ./internal/mistral
GOOGLE_APPLICATION_CREDENTIALS=... go test -tags integration ./internal/tts
```

## What each test pins

There is one row per endpoint the code calls. A unit test in the default suite
(`TestEveryEndpointHasAnIntegrationTest`) fails when an endpoint constant has no
row here, when a row names a test that does not exist, or when a request is made
without a named endpoint, so a new endpoint cannot ship without a real test.

| Package | Endpoint | Tests | Pins | Cost |
|---|---|---|---|---|
| mistral | POST `/v1/chat/completions` | TestChatCompletionsWithATinySchema, TestChatCompletionsAcceptsThePromptSchemaInStrictMode, TestChatCompletionsRejectsAnUnknownModelAsACallError, TestBadKeyOnChatIsAnAuthError | A json_schema answer comes back as a JSON string and parses; usage reports tokens and the model; the real `prompt.Schema()` is accepted in strict mode and `prompt.ParseCards` reads the answer; an unknown model is a `CallError`; a bad key is an `AuthError`. | The first two use `mistral-small-latest`: about 60 and 500 tokens. The last two are rejected before any token is used. |
| mistral | GET `/v1/models` | TestModelsEndpointAcceptsTheKeyAndRejectsABadOne | The key check `doctor` relies on: 200 for a good key, an authentication error for a bad one. | Free. |
| tts | GET `/v1/voices` | TestGoogleVoicesListsTheArabicVoicesAndTheDefaultExists, TestGoogleRejectsABadTokenAsAnAuthError | The Arabic voice list decodes and contains the default voice; a bad token is an authentication error. | Free. |
| tts | POST `/v1/text:synthesize` | TestGoogleSynthesizeReturnsMP3 | The request body is accepted for the default voice and the reply decodes to an MP3. | One word, 3 letters. |

The whole suite costs a fraction of a cent: roughly 600 tokens of
`mistral-small-latest` and six characters of speech. The tests never loop and
never retry a failed call, so a broken credential or a changed API costs
nothing more than the first call.
