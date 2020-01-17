package preflight._2_pods

import input["k8s/pods"] as pods

# 2.1 Resources

# 2.1.1 CPU requests set
preflight_2_1_1[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.requests.cpu
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a cpu request", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_2_1_1[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.requests.cpu
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a cpu request", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.1.2 Memory requests set
preflight_2_1_2[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.requests.memory
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a memory request", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_2_1_2[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.requests.memory
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a memory request", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.1.3 CPU limits set
preflight_2_1_3[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.limits.cpu
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a cpu limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_2_1_3[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.limits.cpu
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a cpu limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.1.4 Memory limits set
preflight_2_1_4[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# test if the limits are not set
	not container.resources.limits.memory
	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing a memory limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_2_1_4[message] {
	# find all initContainers in all pods
	pod := pods.items[_]
	container := pod.spec.initContainers[_]
	# test if the limits are not set
	not container.resources.limits.memory
	# bind a message for reporting
	message := sprintf("init container '%s' in pod '%s' in namespace '%s' is missing a memory limit", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.1.5 Guaranteed QoS
preflight_2_1_5[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	{ container.resources.requests == {},
	container.resources.limits == {},
	container.resources.requests != container.resources.limits } & { true } != set()

	message := sprintf("container '%s' in pod '%s' in namespace '%s' is not Guaranteed QoS", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_2_1_5[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.resources.requests

	message := sprintf("container '%s' in pod '%s' in namespace '%s' is not Guaranteed QoS", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_2_1_5[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.resources.limits

	message := sprintf("container '%s' in pod '%s' in namespace '%s' is not Guaranteed QoS", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.2 Monitoring

# 2.2.1 Liveness probe set
preflight_2_2_1[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	container.livenessProbe == {}

	message := sprintf("livenessProbe not set in container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_2_2_1[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.livenessProbe

	message := sprintf("livenessProbe not set in container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.2.2 Readiness probe set
preflight_2_2_2[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	container.readinessProbe == {}
	message := sprintf("readinessProbe not set in container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}

preflight_2_2_2[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.readinessProbe

	message := sprintf("readinessProbe not set in container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.2.3 Liveness and readiness probes are different
preflight_2_2_3[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	r := container.readinessProbe
	r != {}
	l := container.livenessProbe
	l != {}

	l == r

	message := sprintf("container '%s' in pod '%s' in namespace '%s' has equal probes", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.3 Images

# 2.3.1 imagePullPolicy is ifNotPresent
preflight_2_3_1[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	container.imagePullPolicy != "IfNotPresent"

	message := sprintf("imagePullPolicy is not IfNotPresent for container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}
preflight_2_3_1[message] {
	pod := pods.items[_]
	container := pod.spec.containers[_]
	not container.imagePullPolicy

	message := sprintf("imagePullPolicy is not IfNotPresent for container '%s' in pod '%s' in namespace '%s'", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.3.2 Image has explicit tag or SHA
preflight_2_3_2[message] {
	# find all containers in all pods
	pod := pods.items[_]
	container := pod.spec.containers[_]
	# validate that the image value contains an explicit tag
	{ re_match(`latest$`, container.image),
	re_match(`^[^:]+$`, container.image) } & { true } != set()

	# bind a message for reporting
	message := sprintf("container '%s' in pod '%s' in namespace '%s' is missing an explicit image tag", [container.name, pod.metadata.name, pod.metadata.namespace])
}

# 2.4 Namespaces

# 2.4.1 Deployments across multiple namespaces
preflight_2_4_1[message] {
	pod := pods.items[_]
	# Don't output the namespace too, it's obviously in the 'default' namespace
	pod_name = pod.metadata.name
	pod.metadata.namespace == "default"

	message := sprintf("pod '%s' is running in default namespace", [pod.metadata.name])
}
