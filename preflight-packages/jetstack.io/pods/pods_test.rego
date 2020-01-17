package preflight._2_pods

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
test_2_1_1_no_pods {
	output := preflight_2_1_1 with input as pods([])
	assert_allowed(output)
}
test_2_1_1_cpu_requests_set {
	output := preflight_2_1_1 with input as pods(
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
test_2_1_1_init_containers_unset {
	output := preflight_2_1_1 with input as pods(
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
test_2_1_1_init_containers_set {
	output := preflight_2_1_1 with input as pods(
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
test_2_1_1_cpu_requests_unset {
	output := preflight_2_1_1 with input as pods(
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
test_2_1_1_cpu_requests_some_unset {
	output := preflight_2_1_1 with input as pods(
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

# 2.1.2 Memory requests set
test_2_1_2_no_pods {
	output := preflight_2_1_2 with input as pods([])
	assert_allowed(output)
}
test_2_1_2_memory_requests_set {
	output := preflight_2_1_2 with input as pods(
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
test_2_1_2_init_containers_unset {
	output := preflight_2_1_2 with input as pods(
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
test_2_1_2_init_containers_set {
	output := preflight_2_1_2 with input as pods(
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
test_2_1_2_memory_requests_unset {
	output := preflight_2_1_2 with input as pods(
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
test_2_1_2_memory_requests_some_unset {
	output := preflight_2_1_2 with input as pods(
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


# 2.1.3 CPU limits set
test_2_1_3_no_pods {
	output := preflight_2_1_3 with input as pods([])
	assert_allowed(output)
}
test_2_1_3_cpu_limits_set {
	output := preflight_2_1_3 with input as pods(
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
test_2_1_3_init_containers_unset {
	output := preflight_2_1_3 with input as pods(
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
test_2_1_3_init_containers_set {
	output := preflight_2_1_3 with input as pods(
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
test_2_1_3_cpu_limits_unset {
	output := preflight_2_1_3 with input as pods(
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
test_2_1_3_cpu_limits_some_unset {
	output := preflight_2_1_3 with input as pods(
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

# 2.1.4 Memory limits set
test_2_1_4_no_pods {
	output := preflight_2_1_4 with input as pods([])
	assert_allowed(output)
}
test_2_1_4_memory_limits_set {
	output := preflight_2_1_4 with input as pods(
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
test_2_1_4_init_containers_unset {
	output := preflight_2_1_4 with input as pods(
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
test_2_1_4_init_containers_set {
	output := preflight_2_1_4 with input as pods(
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
test_2_1_4_memory_limits_unset {
	output := preflight_2_1_4 with input as pods(
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
test_2_1_4_memory_limits_some_unset {
	output := preflight_2_1_4 with input as pods(
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

# 2.1.5 Guarantead QoS
test_2_1_5_no_pods {
	output := preflight_2_1_5 with input as pods([])
	assert_allowed(output)
}
test_2_1_5_requests_limits_equal {
	output := preflight_2_1_5 with input as pods(
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
test_2_1_5_requests_missing {
	output := preflight_2_1_5 with input as pods(
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
test_2_1_5_limits_missing {
	output := preflight_2_1_5 with input as pods(
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
test_2_1_5_requests_limits_absent {
	output := preflight_2_1_5 with input as pods(
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
test_2_1_5_requests_limits_not_set {
	output := preflight_2_1_5 with input as pods(
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
test_2_1_5_requests_limits_blank {
	output := preflight_2_1_5 with input as pods(
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
test_2_1_5_requests_limits_some_not_set {
	output := preflight_2_1_5 with input as pods(
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
test_2_1_5_requests_limits_all_set {
	output := preflight_2_1_5 with input as pods(
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

# 2.2 Monitoring

# 2.2.1 Liveness probe set
test_2_2_1_no_pods {
	output := preflight_2_2_1 with input as pods([])
	assert_allowed(output)
}
test_2_2_1_liveness_probe_set {
	output := preflight_2_2_1 with input as pods(
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
test_2_2_1_liveness_probe_unset {
	output := preflight_2_2_1 with input as pods(
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
test_2_2_1_liveness_probe_empty {
	output := preflight_2_2_1 with input as pods(
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
test_2_2_1_liveness_probe_some_unset {
	output := preflight_2_2_1 with input as pods(
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
test_2_2_1_liveness_probe_all_set {
	output := preflight_2_2_1 with input as pods(
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

# 2.2.2 Readiness probe set
test_2_2_2_no_pods {
	output := preflight_2_2_2 with input as pods([])
	assert_allowed(output)
}
test_2_2_2_readiness_probe_set {
	output := preflight_2_2_2 with input as pods(
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
test_2_2_2_readiness_probe_unset {
	output := preflight_2_2_2 with input as pods(
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
test_2_2_2_readiness_probe_empty {
	output := preflight_2_2_2 with input as pods(
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
test_2_2_2_readiness_probe_some_unset {
	output := preflight_2_2_2 with input as pods(
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
test_2_2_2_readiness_probe_all_set {
	output := preflight_2_2_2 with input as pods(
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

# 2.2.3
test_2_2_3_no_pods {
	output := preflight_2_2_3 with input as pods([])
	assert_allowed(output)
}
test_2_2_3_liveness_readiness_not_equal {
	output := preflight_2_2_3 with input as pods(
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
test_2_2_3_liveness_readiness_equal {
	output := preflight_2_2_3 with input as pods(
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
test_2_2_3_liveness_readiness_some_pods_equal {
	output := preflight_2_2_3 with input as pods(
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

# 2.3 Images

# 2.3.1 Image pull policy is ifNotPresent
test_2_3_1_no_pods {
	output := preflight_2_3_1 with input as pods([])
	assert_allowed(output)
}
test_2_3_1_no_pull_policy {
	output := preflight_2_3_1 with input as pods(
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
test_2_3_1_always {
	output := preflight_2_3_1 with input as pods(
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
test_2_3_1_if_not_present {
	output := preflight_2_3_1 with input as pods(
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
test_2_3_1_some_pods_always {
	output := preflight_2_3_1 with input as pods(
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
test_2_3_1_all_pods_ifnotpresent {
	output := preflight_2_3_1 with input as pods(
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

# 2.3.2 Image has explicit tag or SHA
test_2_3_2_no_pods {
	output := preflight_2_3_2 with input as pods([])
	assert_allowed(output)
}
test_2_3_2_named_tag {
	output := preflight_2_3_2 with input as pods(
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
test_2_3_2_latest_tag {
	output := preflight_2_3_2 with input as pods(
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
test_2_3_2_missing_tag {
	output := preflight_2_3_2 with input as pods(
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
test_2_3_2_sha {
	output := preflight_2_3_2 with input as pods(
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
test_2_3_2_some_pods_latest {
	output := preflight_2_3_2 with input as pods(
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
test_2_3_2_all_pods_complient {
	output := preflight_2_3_2 with input as pods(
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

# 2.4 Namespaces

# 2.4.1 Pods across multiple namespaces
test_2_4_1_no_pods {
	output := preflight_2_4_1 with input as pods([])
	assert_allowed(output)
}
test_2_4_1_no_default {
	output := preflight_2_4_1 with input as pods(
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
test_2_4_1_default {
	output := preflight_2_4_1 with input as pods(
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
test_2_4_1_multiple_no_default {
	output := preflight_2_4_1 with input as pods(
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
test_2_4_1_multiple_default {
	output := preflight_2_4_1 with input as pods(
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
