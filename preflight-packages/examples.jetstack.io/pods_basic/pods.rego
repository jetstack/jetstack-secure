package pods

# See https://github.com/jetstack/preflight/blob/master/docs/datagatherers/k8s_pods.md for more details
import input["k8s/pods"] as pods

# Rule 'container_cpu_limit'
default preflight_container_cpu_limit = false
preflight_container_cpu_limit {
        count(containers_without_cpu_limits) == 0
        count(init_containers_without_cpu_limits) == 0
}
format_container(pod, container) = name {
        name := {"namespace": pod.metadata.namespace,
                 "pod": pod.metadata.name,
                 "container": container.name}
}
cpu_limit_unset(container) {
        not container.resources.limits.cpu
}
containers_without_cpu_limits[container_name] {
        pod := pods.items[_]
        container := pod.spec.containers[_]
        container_name = format_container(pod, container)
        cpu_limit_unset(container)
}
init_containers_without_cpu_limits[container_name] {
        pod := pods.items[_]
        container := pod.spec.initContainers[_]
        container_name = format_container(pod, container)
        cpu_limit_unset(container)
}

# Rule 'container_mem_limit'
default preflight_container_mem_limit = false
preflight_container_mem_limit {
        count(containers_without_memory_limits) == 0
        count(init_containers_without_memory_limits) == 0
}
memory_limit_unset(container) {
        not container.resources.limits.memory
}
containers_without_memory_limits[container_name] {
        pod := pods.items[_]
        container := pod.spec.containers[_]
        container_name = format_container(pod, container)
        memory_limit_unset(container)
}
init_containers_without_memory_limits[container_name] {
        pod := pods.items[_]
        container := pod.spec.initContainers[_]
        container_name = format_container(pod, container)
        memory_limit_unset(container)
}

# Rule 'explicit_image_tag'
default preflight_explicit_image_tag = false
preflight_explicit_image_tag {
        count(containers_without_explicit_tag) == 0
}
explicit_tag(container) {
        not re_match(`^.*:latest$`, container.image)
        re_match(`^.*:.*$`, container.image)
}
containers_without_explicit_tag[container_name] {
        pod := pods.items[_]
        container := pod.spec.containers[_]
        container_name = format_container(pod, container)
        not explicit_tag(container)
}