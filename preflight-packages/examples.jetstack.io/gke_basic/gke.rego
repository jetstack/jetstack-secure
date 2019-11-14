package gke_basic

# See https://github.com/jetstack/preflight/blob/master/docs/datagatherers/gke.md for more details
import input.gke.Cluster as gke

# Rule 'private_cluster'
default preflight_private_cluster = false
preflight_private_cluster {
        gke.privateClusterConfig.enablePrivateNodes == true
}

# Rule 'basic_auth_disabled'
default preflight_basic_auth_disabled = false
preflight_basic_auth_disabled {
        not gke.masterAuth.username
        not gke.masterAuth.password
}

# Rule 'abac_disabled'
default preflight_abac_disabled = false
preflight_abac_disabled {
        not gke.legacyAbac.enabled == true
}

# Rule 'k8s_master_up_to_date'
default preflight_k8s_master_up_to_date = false
preflight_k8s_master_up_to_date {
        re_match(`^1\.1[234].*$`, gke.currentMasterVersion)
}

# Rule 'k8s_nodes_up_to_date'
default preflight_k8s_nodes_up_to_date = false
preflight_k8s_nodes_up_to_date {
        count(node_pools_old_version) == 0
}
node_pools_old_version[name] {
        np := gke.nodePools[_]
        name := np.name
        not node_pools_current_version[name]
}
node_pools_current_version[name] {
        np := gke.nodePools[_]
        name := np.name
        re_match(`^1\.1[234].*$`, np.version)
}
