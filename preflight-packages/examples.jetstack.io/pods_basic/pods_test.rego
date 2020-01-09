package pods

import data.test_utils as test

pods(x) = y { y := {"k8s/pods": {"items": x }} }

# Rule 'container_cpu_limit'
test_container_cpu_limit_no_pods {
	output := preflight_container_cpu_limit with input as pods([])

	# no validation messages should be returned
	test.assert_allowed(output)
}
test_container_cpu_limit_cpu_limits_set {
	output := preflight_container_cpu_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one",
				 "resources":{"limits":{"cpu": "500m"}}},
		{"name": "container-two",
				 "resources":{"limits":{"cpu": "100m"}}}
	]}}])

	# no validation messages should be returned
	test.assert_allowed(output)
}
test_container_cpu_limit_init_containers_unset {
	output := preflight_container_cpu_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{
				 "initContainers":[
				{"name": "init-one"}
				 ],
				 "containers":[
				{"name": "container-one",
						 "resources":{"limits":{"cpu": "500m"}}},
				 ]
		}}])

	# ensure validation message is returned
	test.assert_violates(output, {"init container 'init-one' in pod 'foo' in namespace 'default' is missing a cpu limit"})
}
test_container_cpu_limit_init_containers_set {
	output := preflight_container_cpu_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{
				 "initContainers":[
				{"name": "init-one",
						 "resources": {"limits": {"cpu": "100m"}}}
				 ],
				 "containers":[
				{"name": "container-one",
						 "resources":{"limits":{"cpu": "500m"}}},
				 ]
		}}])

	# no validation messages should be returned
	test.assert_allowed(output)
}
test_container_cpu_limit_cpu_limits_unset {
	output := preflight_container_cpu_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one"}
	]}}])

	# ensure validation message is returned
	test.assert_violates(output, {"container 'container-one' in pod 'foo' in namespace 'default' is missing a cpu limit"})
}
test_container_cpu_limit_cpu_limits_some_unset {
	output := preflight_container_cpu_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one",
				 "resources":{"limits":{"cpu": "500m"}}},
		{"name": "container-two"}
	]}}])

	# ensure validation message is returned
	test.assert_violates(output, {"container 'container-two' in pod 'foo' in namespace 'default' is missing a cpu limit"})
}
test_container_cpu_limit_cpu_limits_many_unset {
	output := preflight_container_cpu_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{
				 "initContainers":[
				{"name": "init-one",
						 "resources": {}}
				 ],
				 "containers":[
				{"name": "container-one",
						 "resources":{}},
				 ]
		}}])

	# ensure validation message for each container is returned
	test.assert_violates(output, {
		"container 'container-one' in pod 'foo' in namespace 'default' is missing a cpu limit",
		"init container 'init-one' in pod 'foo' in namespace 'default' is missing a cpu limit"
	})
}

# Rule 'container_mem_limit'
test_container_mem_limit_no_pods {
	output := preflight_container_mem_limit with input as pods([])

	# no validation messages should be returned
	test.assert_allowed(output)
}
test_container_mem_limit_memory_limits_set {
	output := preflight_container_mem_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one",
				 "resources":{"limits":{"memory": "500m"}}},
		{"name": "container-two",
				 "resources":{"limits":{"memory": "100m"}}}
	]}}])

	# no validation messages should be returned
	test.assert_allowed(output)
}
test_container_mem_limit_init_containers_unset {
	output := preflight_container_mem_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{
				 "initContainers":[
				{"name": "init-one"}
				 ],
				 "containers":[
				{"name": "container-one",
						 "resources":{"limits":{"memory": "500m"}}},
				 ]
		}}])

	test.assert_violates(output, {
		"init container 'init-one' in pod 'foo' in namespace 'default' is missing a memory limit"
	})
}
test_container_mem_limit_init_containers_set {
	output := preflight_container_mem_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{
				 "initContainers":[
				{"name": "init-one",
						 "resources": {"limits": {"memory": "100m"}}}
				 ],
				 "containers":[
				{"name": "container-one",
						 "resources":{"limits":{"memory": "500m"}}},
				 ]
		}}])

	# no validation messages should be returned
	test.assert_allowed(output)
}
test_container_mem_limit_memory_limits_unset {
	output := preflight_container_mem_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one"}
	]}}])

	test.assert_violates(output, {
		"container 'container-one' in pod 'foo' in namespace 'default' is missing a memory limit"
	})
}
test_container_mem_limit_memory_limits_some_unset {
	output := preflight_container_mem_limit with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one",
				 "resources":{"limits":{"memory": "500m"}}},
		{"name": "container-two"}
	]}}])

	test.assert_violates(output, {
		"container 'container-two' in pod 'foo' in namespace 'default' is missing a memory limit"
	})
}

# Rule 'explicit_image_tag'
test_explicit_image_tag_no_pods {
	output := preflight_explicit_image_tag with input as pods([])
	test.assert_allowed(output)
}
test_explicit_image_tag_named_tag {
	output := preflight_explicit_image_tag with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
				{"name": "container-one",
				 "image": "gcr.io/my-project/my-image:v0.1"}
	]}}])

	test.assert_allowed(output)
}
test_explicit_image_tag_latest_tag {
	output := preflight_explicit_image_tag with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
				{"name": "container-one",
				 "image": "gcr.io/my-project/my-image:latest"}
	]}}])

	test.assert_violates(output, {
		"container 'container-one' in pod 'foo' in namespace 'default' is missing an explicit image tag"
		})
}
test_explicit_image_tag_missing_tag {
	output := preflight_explicit_image_tag with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
				{"name": "container-one",
				 "image": "gcr.io/my-project/my-image"}
	]}}])

	test.assert_violates(output, {
		"container 'container-one' in pod 'foo' in namespace 'default' is missing an explicit image tag"
		})
}
test_explicit_image_tag_sha {
	output := preflight_explicit_image_tag with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
				{"name": "container-one",
				 "image": "gcr.io/my-project/my-image@sha256:4bdd623e848417d96127e16037743f0cd8b528c026e9175e22a84f639eca58ff"}
	]}}])

	test.assert_allowed(output)
}
test_explicit_image_tag_some_pods_latest {
	output := preflight_explicit_image_tag with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one",
				 "image": "gcr.io/my-project/my-image:latest"}
	]}},
		{"metadata": {
				"name": "bar",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one",
				 "image": "gcr.io/my-project/my-image:v0.2"}
		]}}
		])

	test.assert_violates(output, {
			"container 'container-one' in pod 'foo' in namespace 'default' is missing an explicit image tag"
		})
}
test_explicit_image_tag_all_pods_compliant {
	output := preflight_explicit_image_tag with input as pods([
		{"metadata": {
				"name": "foo",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one",
				"image": "gcr.io/my-project/my-image:v0.2"}
	]}},
		{"metadata": {
				"name": "bar",
				"namespace": "default"
		 },
		 "spec":{"containers":[
		{"name": "container-one",
				"image": "gcr.io/my-project/another-image:v0.3"}
		]}}
		])

	test.assert_allowed(output)
}
