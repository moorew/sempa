# Using Tailscale Aperture for AI

Sempa's AI features (quick-add parsing, planning, summaries, email-title cleanup
and the rest) normally run on an Ollama model on your own server. If you run
[Tailscale Aperture](https://tailscale.com/docs/aperture), Sempa can use any
model Aperture serves instead: Claude, Gemini, OpenAI, or a model hosted
elsewhere on your tailnet.

## How it works

- **No keys in Sempa.** Aperture sits on your tailnet and recognises the Sempa
  server by its tailnet identity. Sempa sends requests with no credential at all;
  whatever Aperture's grants allow that identity is what Sempa can use.
- **It asks Aperture what's available.** Sempa reads `GET /v1/models` and lists
  every model Aperture offers this server. For each one it uses the API format
  Aperture says it supports: OpenAI-style `/v1/chat/completions` or
  Anthropic-style `/v1/messages`.
- **One model for everything.** The model you pick runs every AI feature,
  including email-title cleanup on each imported email. Your local Ollama
  settings are kept, so you can switch back any time.

## Set it up

1. In Sempa, open **Settings → Integrations → AI** and turn on **AI assist**.
2. Under **Where the model runs**, choose **Aperture**.
3. Enter your gateway's URL, for example `https://ai.your-tailnet.ts.net`, and
   press **Connect**. Sempa saves it and lists the models you can use.
4. Pick a model. It's saved straight away.
5. Press **Try it**. Sempa sends one short prompt and shows the answer and how
   long it took. If that works, every AI feature will.

You can also seed the URL with `APERTURE_BASE_URL` in `.env.local`. You still pick
the model in Settings.

## Models listed under "need setting up in Aperture"

These are models Aperture knows about but Sempa can't use as things stand:

- **Passthrough models** (`requires_client_auth`). Aperture forwards these to
  your own provider account, so every request has to carry that account's API
  key. Sempa deliberately holds no keys. To use the model, add your provider key
  to Aperture as a regular (non-passthrough) provider; it then shows up as usable.
  When Aperture lists the same model under both kinds of provider, Sempa shows
  each copy separately (as `provider/model`) so you can pick the one Aperture
  holds the key for.
- **Models offered only on an API Sempa doesn't use**, such as the OpenAI
  Responses API or embeddings.

Sempa never offers a "sign in with Claude" (or similar) option. Anthropic's terms
don't allow third-party apps to route requests through Claude.ai subscription
logins, so keys belong in Aperture.

## Troubleshooting

**"Aperture not reachable from this server"**: the Sempa server makes the
requests, not your browser. It has to reach the gateway over the tailnet.

- With the bundled `docker-compose.yml`, Sempa shares the host's network, so it
  reaches the tailnet through the host's own Tailscale. Check the host can open
  the URL: `curl -sS https://ai.your-tailnet.ts.net/v1/models`.
- If the error says the name can't be resolved, Sempa automatically retries
  `*.ts.net` names through Tailscale's own DNS (`100.100.100.100`). If that also
  fails, the host isn't connected to the tailnet.

**Connected, but no usable models**: Aperture's grants decide what each tailnet
identity may use. Grant the identity Aperture sees for the Sempa server. That is
usually the host's owner, or a tag if the host is tagged. An error from Aperture
names the identity it saw.

**A model is listed but "Try it" fails**: Sempa shows Aperture's own error,
which usually says what's wrong (for example, which API the model expects, or a
missing provider key).

## Costs and privacy

With Aperture, prompts leave your server and go to whichever provider serves the
model. Cloud models may charge per request. Email-title cleanup runs once for
each imported email, so pick a small, fast model if you import a lot of mail. The
local Ollama option remains the default, and with it nothing leaves your server.
