# Test Squadron locally

Local testing should let you change a mission, runtime, or integration and inspect
what Squadron actually does before involving production services. Run the built
CLI against a separate configuration directory and keep its vault, database, and
logs with that test. Substitute the external services relevant to the scenario;
a gateway is optional.

## How do I run a mission without credentials?

From the Squadron checkout, with Go and Python 3 installed:

```bash
./scripts/local.sh run
```

This builds the CLI and executes four scenarios through that binary:

| Scenario | What the real runtime must do |
| --- | --- |
| `routing` | Resolve an input, complete classification, run the selected repair task, and leave the other branch unexecuted |
| `delegation` | Call a worker agent, receive its answer, and complete the commander's task |
| `failure` | Return a failure and persist the failed mission and task |
| `missing-input` | Reject a missing required input before calling the model or starting a mission |

The model endpoint is a loopback HTTP server that returns scripted Anthropic
responses, including streamed tool calls. Squadron still parses HCL, builds its
prompts and tools, sends model requests, executes tool calls, and writes its real
SQLite state. The script checks the exit result, number of model requests, and
persisted mission and task states. Unexpected requests or an exhausted script
fail the run, so a missing response cannot silently become a success.

No model key, Slack app, or other external account is needed for these scenarios.
The placeholder key works because the configured model endpoint is the local
stub. A placeholder key pointed at a real model provider would fail authentication.

## Where are the results, and how do I repeat a test?

The default output directory is `.local-testing` inside the checkout. It is
ignored by Git. Set `SQUADRON_TEST_ROOT` to use a separate workspace:

```bash
export SQUADRON_TEST_ROOT="$HOME/code/squadron-local-testing"
./scripts/local.sh run routing
./scripts/local.sh run delegation failure missing-input
```

Each invocation creates a fresh directory under `runs`, so a previous database
cannot make a test appear to pass. Its `result` directory contains:

- `result.json`: checks and observed mission/task states;
- `mission.log`: CLI output;
- `model-requests.json`: the actual prompts, tools, and conversation sent to the stub;
- `scenario.json`: the scripted decisions and expected outcomes;
- `config/`: the rendered HCL and `.squadron` vault and database.

Keep this output local: when adapting a fixture, prompts and logs may contain the
data you supplied. To inspect a failure, start with `result.json`, then compare
the log and model requests against the script. A scripted-model success establishes
runtime behavior for those decisions, not whether a live model would choose them.

## How do I build and test source changes?

```bash
./scripts/local.sh build
./scripts/local.sh test
```

`build` writes the CLI to the test root's `bin` directory. `test` rebuilds it,
runs the configuration, mission, agent, built-in tool, storage, human-input,
gateway-host, and MCP-host package tests, then runs the four CLI scenarios. This
is a focused local check, not every repository test or a production validation.
Use `go test ./...` when the change requires the complete repository suite.

For related repositories, check out the intended branches in sibling directories.
A Go workspace can make the runtime and an integration use the same SDK source
without editing their published module dependencies. For the Slack gateway,
run from the sibling checkouts' parent directory:

```bash
go work init ./squadron ./squadron-gateway-slack
go work edit -replace=github.com/mlund01/squadron-gateway-sdk=./squadron-gateway-sdk
```

Create the workspace once; update an existing workspace instead of running init
again. The SDK is the replacement target, so do not also add it to the `use` list.
Other integrations need their own module paths and dependency overrides.

After updating a branch, rebuild the affected binaries and restart any running
instance. A running process does not adopt a newly built binary automatically.
Record `git rev-parse HEAD` for each participating checkout when sharing results;
include local diffs if those checkouts have uncommitted changes.

## How do I add a scenario?

The runnable fixtures live in `examples/local-testing`. A scenario JSON selects
a mission, supplies its inputs, scripts model turns, and states the expected
outcome. Each turn matches an expected prompt marker and returns either a tool
call or text. The adjacent `squadron.hcl` supplies the configuration; the runner
replaces `__MODEL_URL__` with its temporary loopback endpoint.

To run a fixture from another directory:

```bash
python3 scripts/local-test.py /path/to/fixture/scenario.json \
  --binary .local-testing/bin/squadron \
  --output .local-testing/runs/my-scenario
```

Use an output path that does not exist. The runner refuses to overwrite evidence.
Its current fixture loader copies the adjacent HCL file, not a complete application
configuration tree. The ordered response script suits sequential scenarios;
parallel agent calls require matching responses by request identity instead of
assuming a single call order.

Choose assertions that establish the behavior under test. For example, a routing
scenario should verify which branch executed and which did not, rather than only
checking that the model returned a route string. To test tool payloads, provide a
local substitute for that tool's service and inspect the requests it receives.

## How do I run my own configuration interactively?

Create a disposable configuration directory from the revision you want to test,
then run:

```bash
./scripts/local.sh verify /absolute/path/to/test-config
./scripts/local.sh start /absolute/path/to/test-config
```

The instance runs in the foreground; Ctrl-C stops it. Both commands explicitly use
that configuration's `.squadron` directory, so an inherited `SQUADRON_HOME` cannot
redirect them into another instance's state. `start` initializes its vault if needed
and runs headless. To use the command center instead, run the built CLI with
`engage --foreground --init -c .` from the configuration directory and choose an
unused `--cc-port`.

Review the copied configuration before loading it. `verify` can install plugins
and contact configured MCP services, while `start` can start gateways and schedules.
An isolated database does not isolate external side effects. Point model and
service endpoints at local stubs, replace external tools with test implementations,
and remove automatic triggers or schedules that are outside the scenario. This
setup does not automatically rewrite arbitrary configurations into offline ones.

For a live-model test, store the model key in that test directory's vault and use
the provider's endpoint. For a mixed test, use a real model with stubbed service
calls, or a scripted model with a selected real integration. Document which parts
are real so captured output is not mistaken for evidence about a stubbed component.

## How do integration-specific tests fit in?

Build and install a local integration only when the scenario needs it. A gateway
binary belongs at
`<test-config>/.squadron/gateways/<GOOS>-<GOARCH>/<name>/local/gateway`, with
`version = "local"` in the test configuration. Keep relative checkpoint paths
inside that configuration and start from its directory so saved mappings stay
with the same test state.

[Slack message testing](local-slack-testing.md) is one example: it can record
outgoing Slack API calls without a live workspace, or later connect a synthetic
mission to a test app. Its mock directly exercises gateway handlers and does not
replace the local runtime scenarios above. A rate workflow needs its own tests
for input handoffs, skills, checkpoints, and service calls; a passing gateway
preview cannot establish those behaviors.
