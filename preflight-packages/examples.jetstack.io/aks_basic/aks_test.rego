package aks_basic

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

cluster(x) = y { y := {"aks": {"Cluster": x }} }

# RBAC Enabled
test_rbac_enabled_enabled {
	output := rbac_enabled with input as cluster(
		{
			"properties": {
				"enableRBAC": true
			}
		}
	)
	assert_allowed(output)
}
test_rbac_enabled_disabled {
	output := rbac_enabled with input as cluster(
		{
			"properties": {
				"enableRBAC": false
			}
		}
	)
	assert_violates(output,
		{
			"rbac is not enabled"
		}
	)
}
test_rbac_enabled_missing {
	output := rbac_enabled with input as cluster(
		{}
	)
	assert_violates(output,
		{
			"rbac is not enabled"
		}
	)
}
