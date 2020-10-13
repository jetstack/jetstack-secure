package api

// AgentMetadata is metadata about the agent.
type AgentMetadata struct {
	Version string `json:"version"`
	// ClusterID is the name of the cluster or host where the agent is running.
	// It may send data for other clusters in its datareadings.
	ClusterID string `json:"cluster_id"`
}
