package pods

import input["k8s/pods"] as pods

# Resources

# CPU requests set
cpu_requests_set[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.requests.cpu
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a cpu request", [container.name, pod.metadata.name, pod.metadata.namespace])
}
cpu_requests_set[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.requests.cpu
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a cpu request", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Memory requests set
memory_requests_set[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.requests.memory
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a memory request", [container.name, pod.metadata.name, pod.metadata.namespace])
}
memory_requests_set[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.requests.memory
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a memory request", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# CPU limits set
cpu_limits_set[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.limits.cpu
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a cpu limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}
cpu_limits_set[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.limits.cpu
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a cpu limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Memory limits set
memory_limits_set[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.limits.memory
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a memory limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}
memory_limits_set[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.limits.memory
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a memory limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Guaranteed QoS
guaranteed_qos[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	{ container.resources.requests == {},
	container.resources.limits == {},
	container.resources.requests != container.resources.limits } & { true } != set()
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is not Guaranteed QoS", [container.name, pod.metadata.name, pod.metadata.namespace])
}
guaranteed_qos[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.resources.requests
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is not Guaranteed QoS", [container.name, pod.metadata.name, pod.metadata.namespace])
}
guaranteed_qos[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.resources.limits
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is not Guaranteed QoS", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Monitoring

# Liveness probe set
liveness_probe_set[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	container.livenessProbe == {}
	message := sprintf("livenessProbe not set in container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}
liveness_probe_set[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.livenessProbe
	message := sprintf("livenessProbe not set in container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Readiness probe set
readiness_probe_set[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	container.readinessProbe == {}
	message := sprintf("readinessProbe not set in container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}
readiness_probe_set[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.readinessProbe
	message := sprintf("readinessProbe not set in container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Liveness and readiness probes are different
liveness_and_readiness_probes_are_different[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	r := container.readinessProbe
	r != {}
	l := container.livenessProbe
	l != {}
	l == r
	message := sprintf("container '%s' in pod '%s' in namespace '%s' has equal probes", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Images

# imagePullPolicy is ifNotPresent
imagepullpolicy_is_ifnotpresent[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	container.imagePullPolicy != "IfNotPresent"
	message := sprintf("imagePullPolicy is not IfNotPresent for container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}
imagepullpolicy_is_ifnotpresent[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.imagePullPolicy
	message := sprintf("imagePullPolicy is not IfNotPresent for container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Image has explicit tag or SHA
image_has_explicit_tag_or_sha[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# validate that the image value contains an explicit tag
	{ re_match(`latest$`, container.image),
	re_match(`^[^:]+$`, container.image) } & { true } != set()
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing an explicit image tag", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Namespaces

# Deployments across multiple namespaces
deployments_across_multiple_namespaces[message] {
	pod := pods.items[_]
	# Don't output the namespace too, it's obviously in the 'default' namespace
	pod_name = pod.metadata.name
	pod.metadata.namespace == "default"
	message := sprintf("pod '%s' is running in default namespace", [pod.metadata.name])
}
