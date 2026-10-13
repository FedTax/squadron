# Test Slack messages locally

Start with the Slack gateway's mock walkthrough to inspect outgoing messages
without credentials or external posts. It runs the real gateway message handlers
against local HTTP substitutes for Slack and Squadron's mission API. Its scripted
case events exercise the message contract without asking a model to choose tools.

For a later live check, the fixture in
[`examples/slack-smoke/squadron.hcl`](../examples/slack-smoke/squadron.hcl) uses a
small synthetic mission that needs only Squadron, its Slack gateway, and an
Anthropic model. It does not load rate workflows or their external tools.

## Preview the outgoing calls without credentials

After setting up the sibling checkouts and Go workspace below, run from their
parent directory:

```bash
mkdir -p preview
SQUADRON_SLACK_PREVIEW="$PWD/preview/slack-calls.json" \
  go test ./squadron-gateway-slack -run '^TestMockSlackPreview$' -count=1 -v
```

The Slack gateway checkout must include `mock_preview_test.go`. This walkthrough
uses a local Slack HTTP endpoint and a local MCP server, so it needs no vault,
Slack app, model, or Socket Mode connection. It records two entry paths: a mission
that posts first, and a mission started through the slash-command form. Each path
updates the root, posts a reply, asks and resolves choice and free-text questions,
starts a routed run, sends a failure notification, and reloads a checkpoint before
a final root update.

The JSON contains `calls` with the API method, message payload, returned timestamp,
and scenario step, plus `checks` for shared-thread behavior. `chat.postMessage`
without `thread_ts` creates a root; with `thread_ts`, it creates a reply.
`chat.update` changes the message identified by `ts`. The payload's `blocks`
contain Slack's question controls, while `views.open` and `views.update` contain
the start form.

The walkthrough passing means it completed and recorded the requests. Inspect the
`checks` separately: a false result records a behavioral defect without preventing
the rest of the preview from being captured. The regular tests remain separate.
Scripted answers bypass Socket Mode delivery, and scripted mission events do not
prove that a live model or the rate workflow will choose the same actions. Slack
itself may also reject a payload that the permissive local server accepts.

## When are real credentials needed?

The SDK, host, and Slack gateway tests run without Slack or model credentials.
`verify` checks the fixture's configuration and finds the local gateway binary;
it does not authenticate to Slack or call a model. Placeholder values are enough
for that check. They are not enough to execute a mission.

A live `/squadron` form needs real Slack tokens but does not call the model until
you submit it. Running the fixture needs a real Anthropic API key and incurs
model usage. A dummy key will cause the run to fail rather than simulate an agent.

## Build matching source revisions

Use sibling checkouts of `squadron`, `squadron-gateway-sdk`, and
`squadron-gateway-slack`. Check out the branches or PR revisions you intend to
test in each repository first. Run these commands from their parent directory:

```bash
# Create this workspace once, in a directory without an existing go.work.
go work init ./squadron ./squadron-gateway-slack
go work edit -replace=github.com/mlund01/squadron-gateway-sdk=./squadron-gateway-sdk
mkdir -p bin
go build -o bin/squadron ./squadron/cmd/cli
go build -o bin/slack-gateway ./squadron-gateway-slack
go test ./squadron-gateway-slack/... ./squadron/gateway/... ./squadron/mcphost/...
(cd squadron-gateway-sdk && GOWORK=off go test ./...)
```

The workspace replacement makes both executables use the same local SDK, even
when their module files pin a released or forked SDK revision. The SDK is a
replacement target, so do not also add it to the workspace's `use` list.
The workspace file belongs outside the repositories so it does not change their
published dependencies. Go must meet the version required by their module files.

After fetching new branch commits, repeat the builds and tests, reinstall the
gateway binary below, and restart Squadron. An already-running process continues
using its loaded binary. Record each repository's `git rev-parse HEAD` when
reporting a result, because a passing test applies to those exact revisions.

## Give the test its own state

From the same parent directory:

```bash
mkdir -p smoke
cp squadron/examples/slack-smoke/squadron.hcl smoke/squadron.hcl
bin/squadron init -c smoke
platform="$(go env GOOS)-$(go env GOARCH)"
mkdir -p "smoke/.squadron/gateways/$platform/slack/local"
cp bin/slack-gateway "smoke/.squadron/gateways/$platform/slack/local/gateway"
```

`version = "local"` selects this binary instead of a release download. The cache
is inside the test configuration's `.squadron` directory, not `~/.squadron`.
Its vault, database, and Slack checkpoint therefore do not share production run
history or request-to-thread mappings. Do not set `SQUADRON_HOME` or pass a
`--squadron-home` override for this example; either changes where the state lives.

The fixture uses MCP port 18090. Its gateway's `mcp_url` and the host's `port`
must agree so the slash-command form can list and start missions. The same
`mcp_host_secret` authenticates that connection. Generate a random value and
store it in the test vault.

Populate these vault variables using `bin/squadron vars -c smoke set NAME VALUE`:

| Variable | Value |
| --- | --- |
| `anthropic_api_key` | Anthropic API key, or a placeholder for validation only |
| `slack_bot_token` | Test app's bot token (`xoxb-…`) |
| `slack_app_token` | Test app's Socket Mode token (`xapp-…`) |
| `slack_channel_id` | Dedicated test channel ID |
| `mcp_host_secret` | Random secret used only for this local MCP host |

To avoid putting a token in shell history, use a hidden prompt in Bash:

```bash
read -r -s -p 'Slack bot token: ' squadron_test_token
printf '\n'
bin/squadron vars -c smoke set slack_bot_token "$squadron_test_token"
unset squadron_test_token
```

Repeat for each credential. The CLI accepts values as arguments, so they may be
briefly visible to local process inspection. Keep the vault and its key out of
version control. The file vault provider keeps both in `.squadron`, which `init`
adds to the test directory's ignore file.

## Connect the test app

Follow the Slack gateway's README to enable Socket Mode, create `/squadron`,
install the bot with `chat:write` and `commands`, and give its app token
`connections:write`. Invite the bot to the test channel. Use a dedicated test app:
multiple instances connected with the same app token can receive events intended
for one another, so a separate channel alone does not isolate the consumer.

Then validate and start from the test directory:

```bash
bin/squadron verify smoke
cd smoke
../bin/squadron engage --foreground --headless -c .
```

The gateway checkpoint path is relative to the working directory, so start from
`smoke` as shown. Foreground mode keeps logs in the terminal, and Ctrl-C stops the
instance. Headless mode avoids downloading a command-center UI; Slack talks to
the local MCP host directly. Confirm the gateway reports ready before opening
the form. A configuration-validation success alone does not prove the gateway
connected.

## Check the behavior in Slack

1. Run `/squadron` and choose `slack_smoke`. `case_key` should be required;
   `states` and `fail_after_reply` should show defaults; `hidden_marker` should
   not appear. Enter malformed JSON for `states` and confirm the form reports an
   input error without starting a run.
2. Submit a fresh key such as `SLACK-TEST-001`, leaving failure disabled. Expect
   one top-level message, a synthetic reply, a choice question, and a free-text
   question with an Answer button. Answer both. The question messages should
   update to show their answers.
3. The mission routes to `slack_smoke_finish` with the same key. Its first post
   should update the existing root, and its question should stay in that thread.
   Answer it and check that the root shows completion. The requester line should
   survive root updates, while replies should not repeatedly mention the user.
4. Run a fresh case with `fail_after_reply` set to JSON `true`. After the two
   answers, expect an intentional failure notification in the case's thread.
5. Stop and restart the instance, then start `slack_smoke_finish` with a previous
   case key. It should rejoin the saved thread. Use a fresh key for unrelated
   tests; reusing a key deliberately exercises the persisted mapping.

These are acceptance checks, not guarantees that a branch implements all of them.
If a routed run opens a second root, inspect whether the gateway saved both the
run-to-thread and key-to-thread mappings when the first run posted its root.
If an agent skips a requested step, inspect its tool calls before attributing the
result to Slack transport: this fixture uses a live model, not a scripted one.

## How does this relate to the rate workflow configuration?

The fixture checks the transport contract used by `slack_thread`: post the stable
request key before asking a question, update the root with `root = true`, and
leave ordinary replies in the thread. It does not prove that the rate agents load
the skill, claim start events correctly, or produce the required final summary.

Test the rate configuration separately in a disposable copy of its selected
revision. Switch its Slack gateway to `version = "local"`, install the same binary
in that copy's state directory, set a separate checkpoint, and make its MCP URL,
port, and secret agree. A release-pinned gateway can ignore new payload fields,
which makes a valid configuration appear to run while missing the behavior under
test. The full workflows additionally need their configured model, plugin, MCP,
and service credentials. Stubbing those dependencies checks configuration shape;
it cannot establish that the actual workflow succeeds.
