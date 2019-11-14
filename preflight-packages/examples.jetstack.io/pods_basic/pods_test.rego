package pods

pods(x) = y { y := {"k8s/pods": {"items": x }} }

# Rule 'container_cpu_limit'
test_container_cpu_limit_no_pods {
	preflight_container_cpu_limit with input as pods([])
}
test_container_cpu_limit_cpu_limits_set {
	preflight_container_cpu_limit with input as pods([
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
}
test_container_cpu_limit_init_containers_unset {
	not preflight_container_cpu_limit with input as pods([
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
}
test_container_cpu_limit_init_containers_set {
	preflight_container_cpu_limit with input as pods([
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
}
test_container_cpu_limit_cpu_limits_unset {
	not preflight_container_cpu_limit with input as pods([
        {"metadata": {
                "name": "foo",
                "namespace": "default"
         },
         "spec":{"containers":[
		{"name": "container-one"}
	]}}])
}
test_container_cpu_limit_cpu_limits_some_unset {
	not preflight_container_cpu_limit with input as pods([
        {"metadata": {
                "name": "foo",
                "namespace": "default"
         },
         "spec":{"containers":[
		{"name": "container-one",
                 "resources":{"limits":{"cpu": "500m"}}},
		{"name": "container-two"}
	]}}])
}

# Rule 'container_mem_limit'
test_container_mem_limit_no_pods {
	preflight_container_mem_limit with input as pods([])
}
test_container_mem_limit_memory_limits_set {
	preflight_container_mem_limit with input as pods([
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
}
test_container_mem_limit_init_containers_unset {
	not preflight_container_mem_limit with input as pods([
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
}
test_container_mem_limit_init_containers_set {
	preflight_container_mem_limit with input as pods([
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
}
test_container_mem_limit_memory_limits_unset {
	not preflight_container_mem_limit with input as pods([
        {"metadata": {
                "name": "foo",
                "namespace": "default"
         },
         "spec":{"containers":[
		{"name": "container-one"}
	]}}])
}
test_container_mem_limit_memory_limits_some_unset {
	not preflight_container_mem_limit with input as pods([
        {"metadata": {
                "name": "foo",
                "namespace": "default"
         },
         "spec":{"containers":[
		{"name": "container-one",
                 "resources":{"limits":{"memory": "500m"}}},
		{"name": "container-two"}
	]}}])
}

# Rule 'explicit_image_tag'
test_explicit_image_tag_no_pods {
        preflight_explicit_image_tag with input as pods([])
}
test_explicit_image_tag_named_tag {
        preflight_explicit_image_tag with input as pods([
        {"metadata": {
                "name": "foo",
                "namespace": "default"
         },
         "spec":{"containers":[
                {"name": "container-one",
                 "image": "gcr.io/my-project/my-image:v0.1"}
	]}}])
}
test_explicit_image_tag_latest_tag {
        not preflight_explicit_image_tag with input as pods([
        {"metadata": {
                "name": "foo",
                "namespace": "default"
         },
         "spec":{"containers":[
                {"name": "container-one",
                 "image": "gcr.io/my-project/my-image:latest"}
	]}}])
}
test_explicit_image_tag_missing_tag {
        not preflight_explicit_image_tag with input as pods([
        {"metadata": {
                "name": "foo",
                "namespace": "default"
         },
         "spec":{"containers":[
                {"name": "container-one",
                 "image": "gcr.io/my-project/my-image"}
	]}}])
}
test_explicit_image_tag_sha {
        preflight_explicit_image_tag with input as pods([
        {"metadata": {
                "name": "foo",
                "namespace": "default"
         },
         "spec":{"containers":[
                {"name": "container-one",
                 "image": "gcr.io/my-project/my-image@sha256:4bdd623e848417d96127e16037743f0cd8b528c026e9175e22a84f639eca58ff"}
	]}}])
}
test_explicit_image_tag_some_pods_latest {
        not preflight_explicit_image_tag with input as pods([
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
}
test_explicit_image_tag_all_pods_complient {
        preflight_explicit_image_tag with input as pods([
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
}
