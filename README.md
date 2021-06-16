[![release-master](https://github.com/jetstack/preflight/actions/workflows/release-master.yml/badge.svg)](https://github.com/jetstack/preflight/actions/workflows/release-master.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jetstack/preflight.svg)](https://pkg.go.dev/github.com/jetstack/preflight)
[![Go Report Card](https://goreportcard.com/badge/github.com/jetstack/preflight)](https://goreportcard.com/report/github.com/jetstack/preflight)

# Jetstack Secure Agent

The Jetstack Secure Agent is a programme for use with [Jetstack
Secure](https://platform.jetstack.io/). This repository hosts the agent
programme only. It sends data to the [Jetstack Secure
SaaS](https://platform.jetstack.io).

## Installation

Please [review the documentation](https://platform.jetstack.io/docs/agent) for
the agent on to get started.

## Local Execution

To build and run a version from master:

```bash
go run main.go agent --agent-config-file ./path/to/agent/config/file.yaml
```

You can find the example agent file
[here](https://github.com/jetstack/preflight/blob/master/agent.yaml).

You might also want to run a local echo server to monitor requests the agent
sends:

```bash
go run main.go echo
```
