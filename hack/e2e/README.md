# README

A smoke test for venafi-kubernetes-agent.

Demonstrates that the agent can be deployed on a GKE Autopilot cluster and
connect to Venafi Control Plane using workload identity federation.

A unique TLS Secret is generated, using cert-manager, and is expected to be
uploaded to the Venafi Control Plane.
The test ends by polling the Venafi Control Plane until the test certificate
appears there.

The GKE Autopilot cluster is re-used between tests. This strikes a balance
between cost and startup time.
The Venafi deployments are scaled to zero between tests so that Autopilot can
scale the cluster nodes to Zero, to save costs.
The Venafi Pods are configured as "Spot Pods", so that Autopilot creates "Spot
nodes", which further reduces the costs.
