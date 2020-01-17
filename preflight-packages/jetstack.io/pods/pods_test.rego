package preflight.pods

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

pods(x) = y { y := {"k8s/pods": {"items": x }} }

# 2.1 Resources

# 2.1.1 CPU requests set
test_cpu_requests_set_no_pods {
	output := cpu_requests_set with input as pods([])
	assert_allowed(output)
}
test_cpu_requests_set_cpu_requests_set {
	output := cpu_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"requests":{
									"cpu":"500m"
								}
							}
						},
						{
							"name":"container-two",
							"resources":{
								"requests":{
									"cpu":"100m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_cpu_requests_set_init_containers_unset {
	output := cpu_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"initContainers":[
						{
							"name":"init-one"
						}
					],
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"requests":{
									"cpu":"500m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"init container 'init-one' in pod 'foo' in namespace 'default' is missing a cpu request"
		}
	)
}
test_cpu_requests_set_init_containers_set {
	output := cpu_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"initContainers":[
						{
							"name":"init-one",
							"resources":{
								"requests":{
									"cpu":"100m"
								}
							}
						}
					],
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"requests":{
									"cpu":"500m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_cpu_requests_set_cpu_requests_unset {
	output := cpu_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"containers":[
						{
							"name":"container-one"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is missing a cpu request"
		}
	)
}
test_cpu_requests_set_cpu_requests_some_unset {
	output := cpu_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"requests":{
									"cpu":"500m"
								}
							}
						},
						{
							"name":"container-two"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-two' in pod 'foo' in namespace 'default' is missing a cpu request"
		}
	)
}

# Memory requests set
test_memory_requests_set_no_pods {
	output := memory_requests_set with input as pods([])
	assert_allowed(output)
}
test_memory_requests_set_memory_requests_set {
	output := memory_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"requests":{
									"memory":"500m"
								}
							}
						},
						{
							"name":"container-two",
							"resources":{
								"requests":{
									"memory":"100m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_memory_requests_set_init_containers_unset {
	output := memory_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"initContainers":[
						{
							"name":"init-one"
						}
					],
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"requests":{
									"memory":"500m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"init container 'init-one' in pod 'foo' in namespace 'default' is missing a memory request"
		}
	)
}
test_memory_requests_set_init_containers_set {
	output := memory_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"initContainers":[
						{
							"name":"init-one",
							"resources":{
								"requests":{
									"memory":"100m"
								}
							}
						}
					],
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"requests":{
									"memory":"500m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_memory_requests_set_memory_requests_unset {
	output := memory_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"containers":[
						{
							"name":"container-one"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is missing a memory request"
		}
	)
}
test_memory_requests_set_memory_requests_some_unset {
	output := memory_requests_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"requests":{
									"memory":"500m"
								}
							}
						},
						{
							"name":"container-two"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-two' in pod 'foo' in namespace 'default' is missing a memory request"
		}
	)
}


# CPU limits set
test_cpu_limits_set_no_pods {
	output := cpu_limits_set with input as pods([])
	assert_allowed(output)
}
test_cpu_limits_set_cpu_limits_set {
	output := cpu_limits_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"limits":{
									"cpu":"500m"
								}
							}
						},
						{
							"name":"container-two",
							"resources":{
								"limits":{
									"cpu":"100m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_cpu_limits_set_init_containers_unset {
	output := cpu_limits_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"initContainers":[
						{
							"name":"init-one"
						}
					],
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"limits":{
									"cpu":"500m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"init container 'init-one' in pod 'foo' in namespace 'default' is missing a cpu limit"
		}
	)
}
test_cpu_limits_set_init_containers_set {
	output := cpu_limits_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"initContainers":[
						{
							"name":"init-one",
							"resources":{
								"limits":{
									"cpu":"100m"
								}
							}
						}
					],
					"containers":[
						{
							"name":"container-one",
							"resources":{
								"limits":{
									"cpu":"500m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_cpu_limits_set_cpu_limits_unset {
	output := cpu_limits_set with input as pods(
		[
			{
				"metadata":{
					"name":"foo",
					"namespace":"default"
				},
				"spec":{
					"containers":[
						{
							"name":"container-one"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is missing a cpu limit"
		}
	)
}
test_cpu_limits_set_cpu_limits_some_unset {
	output := cpu_limits_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"limits": {
									"cpu": "500m"
								}
							}
						},
						{
							"name": "container-two"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-two' in pod 'foo' in namespace 'default' is missing a cpu limit"
		}
	)
}

# Memory limits set
test_memory_limits_set_no_pods {
	output := memory_limits_set with input as pods([])
	assert_allowed(output)
}
test_memory_limits_set_memory_limits_set {
	output := memory_limits_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"limits": {
									"memory": "500m"
								}
							}
						},
						{
							"name": "container-two",
							"resources": {
								"limits": {
									"memory": "100m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_memory_limits_set_init_containers_unset {
	output := memory_limits_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"initContainers": [
						{
							"name": "init-one"
						}
					],
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"limits": {
									"memory": "500m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"init container 'init-one' in pod 'foo' in namespace 'default' is missing a memory limit"
		}
	)
}
test_memory_limits_set_init_containers_set {
	output := memory_limits_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"initContainers": [
						{
							"name": "init-one",
							"resources": {
								"limits": {
									"memory": "100m"
								}
							}
						}
					],
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"limits": {
									"memory": "500m"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_memory_limits_set_memory_limits_unset {
	output := memory_limits_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is missing a memory limit"
		}
	)
}
test_memory_limits_set_memory_limits_some_unset {
	output := memory_limits_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"limits": {
									"memory": "500m"
								}
							}
						},
						{
							"name": "container-two"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-two' in pod 'foo' in namespace 'default' is missing a memory limit"
		}
	)
}

# Guarantead QoS
test_guaranteed_qos_no_pods {
	output := guaranteed_qos with input as pods([])
	assert_allowed(output)
}
test_guaranteed_qos_requests_limits_equal {
	output := guaranteed_qos with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"requests": {
									"cpu": "500m",
									"memory": "300Mi"
								},
								"limits": {
									"cpu": "500m",
									"memory": "300Mi"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_guaranteed_qos_requests_missing {
	output := guaranteed_qos with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"limits": {
									"cpu": "500m",
									"memory": "300Mi"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is not Guaranteed QoS"
		}
	)
}
test_guaranteed_qos_limits_missing {
	output := guaranteed_qos with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"requests": {
									"cpu": "500m",
									"memory": "300Mi"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is not Guaranteed QoS"
		}
	)
}
test_guaranteed_qos_requests_limits_absent {
	output := guaranteed_qos with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is not Guaranteed QoS"
		}
	)
}
test_guaranteed_qos_requests_limits_not_set {
	output := guaranteed_qos with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is not Guaranteed QoS"
		}
	)
}
test_guaranteed_qos_requests_limits_blank {
	output := guaranteed_qos with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"requests": {},
								"limits": {}
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is not Guaranteed QoS"
		}
	)
}
test_guaranteed_qos_requests_limits_some_not_set {
	output := guaranteed_qos with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {}
						},
						{
							"name": "container-two",
							"resources": {
								"requests": {
									"cpu": "500m",
									"memory": "300Mi"
								},
								"limits": {
									"cpu": "500m",
									"memory": "300Mi"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is not Guaranteed QoS"
		}
	)
}
test_guaranteed_qos_requests_limits_all_set {
	output := guaranteed_qos with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"resources": {
								"requests": {
									"cpu": "500m",
									"memory": "300Mi"
								},
								"limits": {
									"cpu": "500m",
									"memory": "300Mi"
								}
							}
						},
						{
							"name": "container-two",
							"resources": {
								"requests": {
									"cpu": "500m",
									"memory": "300Mi"
								},
								"limits": {
									"cpu": "500m",
									"memory": "300Mi"
								}
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}

# Monitoring

# Liveness probe set
test_liveness_probe_set_no_pods {
	output := liveness_probe_set with input as pods([])
	assert_allowed(output)
}
test_liveness_probe_set_liveness_probe_set {
	output := liveness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"livenessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_liveness_probe_set_liveness_probe_unset {
	output := liveness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"livenessProbe not set in container 'container-one' in pod 'foo' in namespace 'default'"
		}
	)
}
test_liveness_probe_set_liveness_probe_empty {
	output := liveness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"livenessProbe": {}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"livenessProbe not set in container 'container-one' in pod 'foo' in namespace 'default'"
		}
	)
}
test_liveness_probe_set_liveness_probe_some_unset {
	output := liveness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"livenessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						},
						{
							"name": "container-two"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"livenessProbe not set in container 'container-two' in pod 'foo' in namespace 'default'"
		}
	)
}
test_liveness_probe_set_liveness_probe_all_set {
	output := liveness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"livenessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						},
						{
							"name": "container-two",
							"livenessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}

# Readiness probe set
test_readiness_probe_set_no_pods {
	output := readiness_probe_set with input as pods([])
	assert_allowed(output)
}
test_readiness_probe_set_readiness_probe_set {
	output := readiness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"readinessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_readiness_probe_set_readiness_probe_unset {
	output := readiness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"readinessProbe not set in container 'container-one' in pod 'foo' in namespace 'default'"
		}
	)
}
# TODO, is this possible?
test_readiness_probe_set_readiness_probe_empty {
	output := readiness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"readinessProbe": {}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"readinessProbe not set in container 'container-one' in pod 'foo' in namespace 'default'"
		}
	)
}
test_readiness_probe_set_readiness_probe_some_unset {
	output := readiness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"readinessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						},
						{
							"name": "container-two"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"readinessProbe not set in container 'container-two' in pod 'foo' in namespace 'default'"
		}
	)
}
test_readiness_probe_set_readiness_probe_all_set {
	output := readiness_probe_set with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"readinessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						},
						{
							"name": "container-two",
							"readinessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}

# Liveness and readiness probes are different
test_liveness_and_readiness_probes_are_different_no_pods {
	output := liveness_and_readiness_probes_are_different with input as pods([])
	assert_allowed(output)
}
test_liveness_and_readiness_probes_are_different_liveness_readiness_not_equal {
	output := liveness_and_readiness_probes_are_different with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"livenessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							},
							"readinessProbe": {
								"httpGet": {
									"path": "/healthz",
									"port": 8080
								},
								"initialDelaySeconds": 3,
								"periodSeconds": 3
							}
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_liveness_and_readiness_probes_are_different_liveness_readiness_equal {
	output := liveness_and_readiness_probes_are_different with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"livenessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							},
							"readinessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' has equal probes"
		}
	)
}
test_liveness_and_readiness_probes_are_different_liveness_readiness_some_pods_equal {
	output := liveness_and_readiness_probes_are_different with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"livenessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							},
							"readinessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							}
						}
					]
				}
			},
			{
				"metadata": {
					"name": "bar",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"livenessProbe": {
								"exec": {
									"command": [
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds": 5,
								"periodSeconds": 5
							},
							"readinessProbe": {
								"httpGet": {
									"path": "/healthz",
									"port": 8080
								},
								"initialDelaySeconds": 3,
								"periodSeconds": 3
							}
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' has equal probes"
		}
	)
}

# Images

# Image pull policy is ifNotPresent
test_imagepullpolicy_is_ifnotpresent_no_pods {
	output := imagepullpolicy_is_ifnotpresent with input as pods([])
	assert_allowed(output)
}
test_imagepullpolicy_is_ifnotpresent_no_pull_policy {
	output := imagepullpolicy_is_ifnotpresent with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"imagePullPolicy is not IfNotPresent for container 'container-one' in pod 'foo' in namespace 'default'"
		}
	)
}
test_imagepullpolicy_is_ifnotpresent_always {
	output := imagepullpolicy_is_ifnotpresent with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"imagePullPolicy": "Always"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"imagePullPolicy is not IfNotPresent for container 'container-one' in pod 'foo' in namespace 'default'"
		}
	)
}
test_imagepullpolicy_is_ifnotpresent_if_not_present {
	output := imagepullpolicy_is_ifnotpresent with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"imagePullPolicy": "IfNotPresent"
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_imagepullpolicy_is_ifnotpresent_some_pods_always {
	output := imagepullpolicy_is_ifnotpresent with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"imagePullPolicy": "Always"
						}
					]
				}
			},
			{
				"metadata": {
					"name": "bar",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"imagePullPolicy": "IfNotPresent"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"imagePullPolicy is not IfNotPresent for container 'container-one' in pod 'foo' in namespace 'default'"
		}
	)
}
test_imagepullpolicy_is_ifnotpresent_all_pods_ifnotpresent {
	output := imagepullpolicy_is_ifnotpresent with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"imagePullPolicy": "IfNotPresent"
						}
					]
				}
			},
			{
				"metadata": {
					"name": "bar",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"imagePullPolicy": "IfNotPresent"
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}

# Image has explicit tag or SHA
test_image_has_explicit_tag_or_sha_no_pods {
	output := image_has_explicit_tag_or_sha with input as pods([])
	assert_allowed(output)
}
test_image_has_explicit_tag_or_sha_named_tag {
	output := image_has_explicit_tag_or_sha with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"image": "gcr.io/my-project/my-image:v0.1"
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_image_has_explicit_tag_or_sha_latest_tag {
	output := image_has_explicit_tag_or_sha with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"image": "gcr.io/my-project/my-image:latest"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is missing an explicit image tag"
		}
	)
}
test_image_has_explicit_tag_or_sha_missing_tag {
	output := image_has_explicit_tag_or_sha with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"image": "gcr.io/my-project/my-image"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is missing an explicit image tag"
		}
	)
}
test_image_has_explicit_tag_or_sha_sha {
	output := image_has_explicit_tag_or_sha with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"image": "gcr.io/my-project/my-image@sha256:4bdd623e848417d96127e16037743f0cd8b528c026e9175e22a84f639eca58ff"
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}
test_image_has_explicit_tag_or_sha_some_pods_latest {
	output := image_has_explicit_tag_or_sha with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"image": "gcr.io/my-project/my-image:latest"
						}
					]
				}
			},
			{
				"metadata": {
					"name": "bar",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"image": "gcr.io/my-project/my-image:v0.2"
						}
					]
				}
			}
		]
	)
	assert_violates(output,
		{
			"container 'container-one' in pod 'foo' in namespace 'default' is missing an explicit image tag"
		}
	)
}
test_image_has_explicit_tag_or_sha_all_pods_complient {
	output := image_has_explicit_tag_or_sha with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"image": "gcr.io/my-project/my-image:v0.2"
						}
					]
				}
			},
			{
				"metadata": {
					"name": "bar",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"name": "container-one",
							"image": "gcr.io/my-project/another-image:v0.3"
						}
					]
				}
			}
		]
	)
	assert_allowed(output)
}

# Namespaces

# Pods across multiple namespaces
test_deployments_across_multiple_namespaces_no_pods {
	output := deployments_across_multiple_namespaces with input as pods([])
	assert_allowed(output)
}
test_deployments_across_multiple_namespaces_no_default {
	output := deployments_across_multiple_namespaces with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "myapp"
				}
			}
		]
	)
	assert_allowed(output)
}
test_deployments_across_multiple_namespaces_default {
	output := deployments_across_multiple_namespaces with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				}
			}
		]
	)
	assert_violates(output,
		{
			"pod 'foo' is running in default namespace"
		}
	)
}
test_deployments_across_multiple_namespaces_multiple_no_default {
	output := deployments_across_multiple_namespaces with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "myapp"
				}
			},
			{
				"metadata": {
					"name": "bar",
					"namespace": "myotherapp"
				}
			}
		]
	)
	assert_allowed(output)
}
test_deployments_across_multiple_namespaces_multiple_default {
	output := deployments_across_multiple_namespaces with input as pods(
		[
			{
				"metadata": {
					"name": "foo",
					"namespace": "default"
				}
			},
			{
				"metadata": {
					"name": "bar",
					"namespace": "myapp"
				}
			}
		]
	)
	assert_violates(output,
		{
			"pod 'foo' is running in default namespace"
		}
	)
}
