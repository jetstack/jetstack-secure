package gke_basic

# See https://github.com/jetstack/preflight/blob/master/docs/datagatherers/gke.md for more details
import input.gke.Cluster as gke

# Rule 'private_cluster'
preflight_private_cluster[message] {
	not gke.privateClusterConfig.enablePrivateNodes

	message := "cluster does not have private nodes enabled"
}

# Rule 'basic_auth_disabled'
preflight_basic_auth_disabled[message] {
	# masterAuth must be missing or an empty {}
	{ gke.masterAuth } & { null, {}} == set()

	message := "cluster does not have basic auth disabled"
}

# Rule 'abac_disabled'
preflight_abac_disabled[message] {
	gke.legacyAbac.enabled

	message := "cluster has legacy abac enabled"
}

# Rule 'k8s_master_up_to_date'
preflight_k8s_master_up_to_date[message] {
	not gke.currentMasterVersion

	message := "cluster master version is missing"
}
preflight_k8s_master_up_to_date[message] {
	not re_match(`^1\.1[23467].*$`, gke.currentMasterVersion)

	message := "cluster master is not up to date"
}

# Rule 'k8s_nodes_up_to_date'
preflight_k8s_nodes_up_to_date[message] {
	node_pool := gke.nodePools[_]
	not re_match(`^1\.1[234567].*$`, node_pool.version)

	message := sprintf("cluster node pool '%s' is outdated", [node_pool.name])
}

preflight_k8s_nodes_up_to_date[message] {
	node_pool:= gke.nodePools[_]
	not node_pool.version

	message := sprintf("cluster node pool '%s' has no version", [node_pool.name])
}
