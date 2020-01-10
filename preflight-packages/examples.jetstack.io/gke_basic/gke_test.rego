package gke_basic

assert_allowed(output) = output {
	trace(sprintf("GOT: %s", [concat(",", output)]))
	trace("WANT: empty set")
	output == set()
}

assert_violates(output, messages) = output {
	trace(sprintf("GOT: %s", [concat(",", output)]))
	trace(sprintf("WANT: %s", [concat(",", messages)]))

	output == messages
}

cluster(x) = y { y := {"gke": {"Cluster": x }} }

# Rule 'private_cluster'
test_private_cluster_private_cluster_private_nodes_enabled {
	output := preflight_private_cluster with input as
		cluster({"privateClusterConfig":{"enablePrivateNodes": true}})

	assert_allowed(output)
}
test_private_cluster_private_cluster_private_nodes_not_enabled {
	output := preflight_private_cluster with input as
		cluster({"privateClusterConfig":{"enablePrivateNodes": false}})

	assert_violates(output, {
		"cluster does not have private nodes enabled"
		})
}
test_private_cluster_private_cluster_private_nodes_not_set {
	output := preflight_private_cluster with input as cluster({"privateClusterConfig":{}})
	assert_violates(output, {
		"cluster does not have private nodes enabled"
		})
}

# Rule 'basic_auth_disabled'
test_basic_auth_disabled_no_username_and_password {
	output := preflight_basic_auth_disabled with input as cluster({"masterAuth":{}})
	assert_allowed(output)
}
test_basic_auth_disabled_no_master_auth {
	output := preflight_basic_auth_disabled with input as cluster({})
	assert_allowed(output)
}
test_basic_auth_disabled_username_and_password {
	output := preflight_basic_auth_disabled with input as
		cluster({"masterAuth":{"username": "foo", "password": "foobar"}})

	assert_violates(output, {
		"cluster does not have basic auth disabled"
		})
}
test_basic_auth_disabled_username_only {
	output := preflight_basic_auth_disabled with input as
		cluster({"masterAuth":{"username": "foo"}})

	assert_violates(output, {
		"cluster does not have basic auth disabled"
		})
}
test_basic_auth_disabled_password_only {
	output := preflight_basic_auth_disabled with input as
		cluster({"masterAuth":{"password": "foobar"}})

	assert_violates(output, {
		"cluster does not have basic auth disabled"
		})
}

# Rule 'abac_disabled'
test_abac_disabled_legacy_abac_enabled {
	output := preflight_abac_disabled with input as
		cluster({"legacyAbac":{"enabled": true}})

	assert_violates(output, {
		"cluster has legacy abac enabled"
		})
}
test_abac_disabled_legacy_abac_disabled {
	output := preflight_abac_disabled with input as
		cluster({"legacyAbac":{"enabled": false}})

	assert_allowed(output)
}
test_abac_disabled_legacy_abac_empty {
	output := preflight_abac_disabled with input as cluster({"legacyAbac":{}})

	assert_allowed(output)
}
test_abac_disabled_legacy_abac_missing {
	output := preflight_abac_disabled with input as cluster({})

	assert_allowed(output)
}

# Rule 'k8s_master_up_to_date'
test_k8s_master_up_to_date_missing_kubernetes_version {
	output := preflight_k8s_master_up_to_date with input as cluster({})

	assert_violates(output, {
		"cluster master version is missing"
		})
}
test_k8s_master_up_to_date_empty_kubernetes_version {
	output :=  preflight_k8s_master_up_to_date with input as
		cluster({"currentMasterVersion": ""})

	assert_violates(output, {
		"cluster master is not up to date"
		})
}
test_k8s_master_up_to_date_ancient_kubernetes_version {
	output := preflight_k8s_master_up_to_date with input as
		cluster({"currentMasterVersion": "1.11.9-gke.5"})

	assert_violates(output, {
		"cluster master is not up to date"
		})
}
test_k8s_master_up_to_date_modern_kubernetes_version {
	output := preflight_k8s_master_up_to_date with input as
		cluster({"currentMasterVersion": "1.13.6-gke.13"})

	assert_allowed(output)
}

# Rule 'k8s_nodes_up_to_date'
test_k8s_nodes_up_to_date_no_node_pools {
	output := preflight_k8s_nodes_up_to_date with input as
		cluster({"nodePools":[]})
	assert_allowed(output)
}
test_k8s_nodes_up_to_date_pool_no_version {
	output := preflight_k8s_nodes_up_to_date with input as
		cluster({ "nodePools":[{"name": "test-pool"}]})

	assert_violates(output, {
		"cluster node pool 'test-pool' has no version"
		})
}
test_k8s_nodes_up_to_date_old_version {
	output := preflight_k8s_nodes_up_to_date with input as
		cluster({"nodePools":[
			{"name": "test-pool", "version": "1.11.9-gke.5"}
		]})

	assert_violates(output, {
		"cluster node pool 'test-pool' is outdated"
		})
}
test_k8s_nodes_up_to_date_modern_version {
	output := preflight_k8s_nodes_up_to_date with input as
		cluster({"nodePools":[
			{"name": "test-pool", "version": "1.13.6-gke.13"}
		]})

	assert_allowed(output)
}
test_k8s_nodes_up_to_date_multiple_pools_some_incorrect {
	output := preflight_k8s_nodes_up_to_date with input as
		cluster({"nodePools":[
			{"name": "test-pool-1", "version": "1.13.6-gke.13"},
			{"name": "test-pool-2","version": "1.11.9-gke.5"}
		]})

	assert_violates(output, {
		"cluster node pool 'test-pool-2' is outdated"
		})
}
test_k8s_nodes_up_to_date_multiple_pools_all_correct {
	output := preflight_k8s_nodes_up_to_date with input as
		cluster({"nodePools":[
			{"name": "test-pool-1", "version": "1.13.6-gke.13"},
			{"name": "test-pool-2","version": "1.13.6-gke.13"}
		]})

	assert_allowed(output)
}
