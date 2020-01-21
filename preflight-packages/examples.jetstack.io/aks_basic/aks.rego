package aks_basic

# See https://github.com/jetstack/preflight/blob/master/docs/datagatherers/aks.md for more details
import input.aks as aks

# RBAC Enabled
rbac_enabled[message] {
	not aks.Cluster.properties.enableRBAC == true
	message := "rbac is not enabled"
}
