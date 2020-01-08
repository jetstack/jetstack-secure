package aks_basic

# See https://github.com/jetstack/preflight/blob/master/docs/datagatherers/aks.md for more details
import input.aks as aks

# Rule 'rbac_enabled'
default preflight_rbac_enabled = false
preflight_rbac_enabled {
        aks.Cluster.properties.enableRBAC == true
}
