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

# Networking

# Private cluster enabled
test_private_cluster_enabled_private_cluster_private_nodes_enabled {
	output := private_cluster_enabled with input as cluster(
		{
			"privateClusterConfig": {
				"enablePrivateNodes": true
			}
		}
	)
	assert_allowed(output)
}
test_private_cluster_enabled_private_cluster_private_nodes_not_enabled {
	output := private_cluster_enabled with input as cluster(
		{
			"privateClusterConfig": {
				"enablePrivateNodes": false
			}
		}
	)
	assert_violates(output,
		{
			"private cluster has not been enabled"
		}
	)
}
test_private_cluster_enabled_private_cluster_private_nodes_not_set {
	output := private_cluster_enabled with input as cluster(
		{
			"privateClusterConfig": {}
		}
	)
	assert_violates(output,
		{
			"private cluster has not been enabled"
		}
	)
}

# Authentication

# Basic authentication disabled
test_basic_authentication_disabled_no_username_and_password {
	output := basic_authentication_disabled with input as cluster(
		{
			"masterAuth": {}
		}
	)
	assert_allowed(output)
}
test_basic_authentication_disabled_no_master_auth {
	output := basic_authentication_disabled with input as cluster(
		{}
	)
	assert_allowed(output)
}
test_basic_authentication_disabled_username_and_password {
	output := basic_authentication_disabled with input as cluster(
		{
			"masterAuth": {
				"username": "foo",
				"password": "foobar"
			}
		}
	)
	assert_violates(output,

		{
			"basic authentication is enabled with username 'foo'",
			"basic authentication is enabled"
		}
	)
}
test_basic_authentication_disabled_username_only {
	output := basic_authentication_disabled with input as cluster(
		{
			"masterAuth": {
				"username": "foo"
			}
		}
	)
	assert_violates(output,
		{
			"basic authentication is enabled with username 'foo'"
		}
	)
}
test_basic_authentication_disabled_password_only {
	output := basic_authentication_disabled with input as cluster(
		{
			"masterAuth": {
				"password": "foobar"
			}
		}
	)
	assert_violates(output,
		{
			"basic authentication is enabled"
		}
	)
}

# Legacy ABAC disabled
test_legacy_abac_disabled_legacy_abac_enabled {
	output := legacy_abac_disabled with input as cluster(
		{
			"legacyAbac": {
				"enabled": true
			}
		}
	)
	assert_violates(output,
		{
			"legacy ABAC is enabled"
		}
	)
}
test_legacy_abac_disabled_legacy_abac_disabled {
	output := legacy_abac_disabled with input as cluster(
		{
			"legacyAbac": {
				"enabled": false
			}
		}
	)
	assert_allowed(output)
}
test_legacy_abac_disabled_legacy_abac_empty {
	output := legacy_abac_disabled with input as cluster(
		{
			"legacyAbac": {}
		}
	)
	assert_allowed(output)
}
test_legacy_abac_disabled_legacy_abac_missing {
	output := legacy_abac_disabled with input as cluster(
		{}
	)
	assert_allowed(output)
}

# Maintainance

# Kubernetes node version up to date
test_kubernetes_node_version_up_to_date_no_node_pools {
	output := kubernetes_node_version_up_to_date with input as cluster(
		{
			"nodePools": []
		}
	)
	assert_allowed(output)
}
test_kubernetes_node_version_up_to_date_pool_no_version {
	output := kubernetes_node_version_up_to_date with input as cluster(
		{
			"nodePools": [
				{
					"name": "test-pool"
				}
			]
		}
	)
	assert_violates(output,
		{
			"node pool 'test-pool' version is missing"
		}
	)
}
test_kubernetes_node_version_up_to_date_old_version {
	output := kubernetes_node_version_up_to_date with input as cluster(
		{
			"nodePools": [
				{
					"name": "test-pool",
					"version": "1.11.9-gke.5"
				}
			]
		}
	)
	assert_violates(output,
		{
			"node pool 'test-pool' current version '1.11.9-gke.5' not up to date"
		}
	)
}
test_kubernetes_node_version_up_to_date_up_to_date_version {
	output := kubernetes_node_version_up_to_date with input as cluster(
		{
			"nodePools": [
				{
					"name": "test-pool",
					"version": "1.13.6-gke.13"
				}
			]
		}
	)
	assert_allowed(output)
}
test_kubernetes_node_version_up_to_date_multiple_pools_some_old {
	output := kubernetes_node_version_up_to_date with input as cluster(
		{
			"nodePools": [
				{
					"name": "test-pool-1",
					"version": "1.13.6-gke.13"
				},
				{
					"name": "test-pool-2",
					"version": "1.11.9-gke.5"
				}
			]
		}
	)
	assert_violates(output,
		{
			"node pool 'test-pool-2' current version '1.11.9-gke.5' not up to date"
		}
	)
}
test_kubernetes_node_version_up_to_date_multiple_pools_all_old {
	output := kubernetes_node_version_up_to_date with input as cluster(
		{
			"nodePools": [
				{
					"name": "test-pool-1",
					"version": "1.11.9-gke.5"
				},
				{
					"name": "test-pool-2",
					"version": "1.11.9-gke.5"
				}
			]
		}
	)
	assert_violates(output,
		{
			"node pool 'test-pool-1' current version '1.11.9-gke.5' not up to date",
			"node pool 'test-pool-2' current version '1.11.9-gke.5' not up to date"
		}
	)
}
test_kubernetes_node_version_up_to_date_multiple_pools_all_up_to_date {
	output := kubernetes_node_version_up_to_date with input as cluster(
		{
			"nodePools": [
				{
					"name": "test-pool-1",
					"version": "1.13.6-gke.13"
				},
				{
					"name": "test-pool-2",
					"version": "1.13.6-gke.13"
				}
			]
		}
	)
	assert_allowed(output)
}
