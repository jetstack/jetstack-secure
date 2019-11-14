package gke_basic

cluster(x) = y { y := {"gke": {"Cluster": x }} }

# Rule 'private_cluster'
test_private_cluster_private_cluster_private_nodes_enabled {
	preflight_private_cluster with input as cluster({"privateClusterConfig":{"enablePrivateNodes": true}})
}
test_private_cluster_private_cluster_private_nodes_not_enabled {
	not preflight_private_cluster with input as cluster({"privateClusterConfig":{"enablePrivateNodes": false}})
}
test_private_cluster_private_cluster_private_nodes_not_set {
	not preflight_private_cluster with input as cluster({"privateClusterConfig":{}})
}

# Rule 'basic_auth_disabled'
test_basic_auth_disabled_no_username_and_password {
        preflight_basic_auth_disabled with input as cluster({"masterAuth":{}})
}
test_basic_auth_disabled_no_master_auth {
        preflight_basic_auth_disabled with input as cluster({})
}
test_basic_auth_disabled_username_and_password {
        not preflight_basic_auth_disabled with input as cluster({"masterAuth":{"username": "foo",
                                                "password": "foobar"}})
}
test_basic_auth_disabled_username_only {
        not preflight_basic_auth_disabled with input as cluster({"masterAuth":{"username": "foo"}})
}
test_basic_auth_disabled_password_only {
        not preflight_basic_auth_disabled with input as cluster({"masterAuth":{"password": "foobar"}})
}

# Rule 'abac_disabled'
test_abac_disabled_legacy_abac_enabled {
        not preflight_abac_disabled with input as cluster({"legacyAbac":{"enabled": true}})
}
test_abac_disabled_legacy_abac_disabled {
        preflight_abac_disabled with input as cluster({"legacyAbac":{"enabled": false}})
}
test_abac_disabled_legacy_abac_empty {
        preflight_abac_disabled with input as cluster({"legacyAbac":{}})
}
test_abac_disabled_legacy_abac_missing {
        preflight_abac_disabled with input as cluster({})
}

# Rule 'k8s_master_up_to_date'
test_k8s_master_up_to_date_missing_kubernetes_version {
        not preflight_k8s_master_up_to_date with input as cluster({})
}
test_k8s_master_up_to_date_empty_kubernetes_version {
        not preflight_k8s_master_up_to_date with input as cluster({"currentMasterVersion": ""})
}
test_k8s_master_up_to_date_ancient_kubernetes_version {
        not preflight_k8s_master_up_to_date with input as cluster({"currentMasterVersion": "1.11.9-gke.5"})
}
test_k8s_master_up_to_date_modern_kubernetes_version {
        preflight_k8s_master_up_to_date with input as cluster({"currentMasterVersion": "1.13.6-gke.13"})
}

# Rule 'k8s_nodes_up_to_date'
test_k8s_nodes_up_to_date_no_node_pools {
        preflight_k8s_nodes_up_to_date with input as cluster({"nodePools":[]})
}
test_k8s_nodes_up_to_date_pool_no_version {
        not preflight_k8s_nodes_up_to_date with input as cluster({"nodePools":[
                {"name": "test-pool", }
        ]})
}
test_k8s_nodes_up_to_date_old_version {
        not preflight_k8s_nodes_up_to_date with input as cluster({"nodePools":[
                {"name": "test-pool", "version": "1.11.9-gke.5"}
        ]})
}
test_k8s_nodes_up_to_date_modern_version {
        preflight_k8s_nodes_up_to_date with input as cluster({"nodePools":[
                {"name": "test-pool", "version": "1.13.6-gke.13"}
        ]})
}
test_k8s_nodes_up_to_date_multiple_pools_some_incorrect {
        not preflight_k8s_nodes_up_to_date with input as cluster({"nodePools":[
                {"name": "test-pool-1", "version": "1.13.6-gke.13"},
                {"name": "test-pool-2","version": "1.11.9-gke.5"}
        ]})
}
test_k8s_nodes_up_to_date_multiple_pools_all_correct {
        preflight_k8s_nodes_up_to_date with input as cluster({"nodePools":[
                {"name": "test-pool-1", "version": "1.13.6-gke.13"},
                {"name": "test-pool-2","version": "1.13.6-gke.13"}
        ]})
}
