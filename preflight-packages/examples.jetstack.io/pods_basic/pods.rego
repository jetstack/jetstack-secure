package pods

# See https://github.com/jetstack/preflight/blob/master/docs/datagatherers/k8s_pods.md for more details
import input["k8s/pods"] as pods

# Rule 'container_cpu_limit'
preflight_container_cpu_limit[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.limits.cpu
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a cpu limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_container_cpu_limit[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.limits.cpu
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a cpu limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Rule 'container_mem_limit'
preflight_container_mem_limit[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.limits.memory
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a memory limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_container_mem_limit[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.limits.memory
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a memory limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# Rule 'explicit_image_tag'
preflight_explicit_image_tag[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# validate that the image value contains an explicit tag
	{ re_match(`latest$`, container.image),
	re_match(`^[^:]+$`, container.image) } & { true } != set()

	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing an explicit image tag", [container.name, pod.metadata.name, pod.metadata.namespace])
}

preflight_explicit_image_tag[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# validate that the image value contains an explicit tag
	{ re_match(`latest$`, container.image),
	re_match(`^[^:]+$`, container.image) } & { true } != set()

	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing an explicit image tag", [container.name, pod.metadata.name, pod.metadata.namespace])
}
