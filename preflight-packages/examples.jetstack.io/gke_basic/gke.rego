package gke_basic

# See https://github.com/jetstack/preflight/blob/master/docs/datagatherers/gke.md for more details
import input.gke.Cluster as gke

# Networking

# Private cluster enabled
private_cluster_enabled[message] {
	not gke.privateClusterConfig.enablePrivateNodes == true
	message := "private cluster has not been enabled"
}

# Authentication

# Basic authentication disabled
basic_authentication_disabled[message] {
	gke.masterAuth.username
	message := sprintf("basic authentication is enabled with username '%s'", [gke.masterAuth.username])
}
basic_authentication_disabled[message] {
	gke.masterAuth.password
	message := "basic authentication is enabled"
}

# Legacy ABAC disabled
legacy_abac_disabled[message] {
	gke.legacyAbac.enabled == true
	message := "legacy ABAC is enabled"
}

# Maintainance

# Kubernetes master version up to date
Kubernetes_master_version_up_to_date[message] {
	not gke.currentMasterVersion == ""
	not re_match(`^1\.1[34567].*$`, gke.currentMasterVersion)
	message := sprintf("current master version '%s' is not up to date", [gke.currentMasterVersion])
}
Kubernetes_master_version_up_to_date[message] {
	gke.currentMasterVersion == ""
	message := "current master version is missing"
}
Kubernetes_master_version_up_to_date[message] {
	not gke.currentMasterVersion
	message := "current master version is missing"
}

# Kubernetes node version up to date
kubernetes_node_version_up_to_date[message] {
	np := gke.nodePools[_]
	not np.version == ""
	not re_match(`^1\.1[34567].*$`, np.version)
	message := sprintf("node pool '%s' current version '%s' not up to date", [np.name, np.version])
}
kubernetes_node_version_up_to_date[message] {
	np := gke.nodePools[_]
	np.version == ""
	message := sprintf("node pool '%s' version is missing", [np.name])
}
kubernetes_node_version_up_to_date[message] {
	np := gke.nodePools[_]
	not np.version
	message := sprintf("node pool '%s' version is missing", [np.name])
}
