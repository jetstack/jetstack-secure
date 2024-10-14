package k8s

func FilterSecret(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})

	if v, ok := in["kind"]; ok {
		out["kind"] = v
	}

	if v, ok := in["apiVersion"]; ok {
		out["apiVersion"] = v
	}

	if v, ok := in["metadata"]; ok {
		out["metadata"] = FilterMetadata(v)
	}

	if v, ok := in["type"]; ok {
		out["type"] = v
	}

	if data, ok := in["data"]; ok {
		out["data"] = FilterData(data)
	}

	return out
}

func FilterMetadata(in interface{}) map[string]interface{} {
	metadata, ok := in.(map[string]interface{})
	if !ok {
		return nil // Most likely a programming error.
	}

	out := make(map[string]interface{})
	if annotations, ok := metadata["annotations"]; ok {
		out["annotations"] = annotations
	}

	if labels, ok := metadata["labels"]; ok {
		out["labels"] = labels
	}

	if name, ok := metadata["name"]; ok {
		out["name"] = name
	}

	if namespace, ok := metadata["namespace"]; ok {
		out["namespace"] = namespace
	}

	if ownerReferences, ok := metadata["ownerReferences"]; ok {
		out["ownerReferences"] = ownerReferences
	}

	if selfLink, ok := metadata["selfLink"]; ok {
		out["selfLink"] = selfLink
	}

	if uid, ok := metadata["uid"]; ok {
		out["uid"] = uid
	}

	return out
}

func FilterData(in interface{}) map[string]interface{} {
	data, ok := in.(map[string]interface{})
	if !ok {
		return nil // Most likely a programming error.
	}

	out := make(map[string]interface{})
	if tlsCrt, ok := data["tls.crt"]; ok {
		out["tls.crt"] = tlsCrt
	}

	if caCrt, ok := data["ca.crt"]; ok {
		out["ca.crt"] = caCrt
	}

	return out
}

// Drops the two following fields in-place:
//
//	metadata.managedFields
//	/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration
func DropNoisyFieldsObject(in map[string]interface{}) {
	if metadata, ok := in["metadata"]; ok {
		metadataMap, ok := metadata.(map[string]interface{})
		if ok {
			delete(metadataMap, "managedFields")
			if annotations, ok := metadataMap["annotations"]; ok {
				annotationsMap, ok := annotations.(map[string]interface{})
				if ok {
					delete(annotationsMap, "kubectl.kubernetes.io/last-applied-configuration")
				}
			}
		}
	}
}

// Returns the same object with just the following fields:
//
//	kind
//	apiVersion
//	metadata.annotations
//	metadata.name
//	metadata.namespace
//	metadata.ownerReferences
//	metadata.selfLink
//	metadata.uid
//	spec.host
//	spec.to.kind
//	spec.to.port
//	spec.to.name
//	spec.to.weight
//	spec.tls.termination
//	spec.tls.certificate
//	spec.tls.caCertificate
//	spec.tls.destinationCACertificate
//	spec.tls.insecureEdgeTerminationPolicy
//	spec.wildcardPolicy
//	status
func FilterRoute(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})

	if v, ok := in["kind"]; ok {
		out["kind"] = v
	}

	if v, ok := in["apiVersion"]; ok {
		out["apiVersion"] = v
	}

	if metadata, ok := in["metadata"]; ok {
		out["metadata"] = FilterMetadata(metadata)
	}

	if spec, ok := in["spec"]; ok {
		out["spec"] = FilterRouteSpec(spec)
	}

	if status, ok := in["status"]; ok {
		out["status"] = status
	}

	return out
}

func FilterRouteSpec(in interface{}) map[string]interface{} {
	spec, ok := in.(map[string]interface{})
	if !ok {
		return nil // Most likely a programming error.
	}

	out := make(map[string]interface{})
	if host, ok := spec["host"]; ok {
		out["host"] = host
	}

	if to, ok := spec["to"]; ok {
		toMap, ok := to.(map[string]interface{})
		if ok {
			out["to"] = FilterRouteSpecTo(toMap)
		}
	}

	if tls, ok := spec["tls"]; ok {
		tlsMap, ok := tls.(map[string]interface{})
		if ok {
			out["tls"] = FilterRouteSpecTLS(tlsMap)
		}
	}

	if wildcardPolicy, ok := spec["wildcardPolicy"]; ok {
		out["wildcardPolicy"] = wildcardPolicy
	}

	return out
}

func FilterRouteSpecTo(in interface{}) map[string]interface{} {
	to, ok := in.(map[string]interface{})
	if !ok {
		return nil // Most likely a programming error.
	}

	out := make(map[string]interface{})
	if kind, ok := to["kind"]; ok {
		out["kind"] = kind
	}

	if port, ok := to["port"]; ok {
		out["port"] = port
	}

	if name, ok := to["name"]; ok {
		out["name"] = name
	}

	if weight, ok := to["weight"]; ok {
		out["weight"] = weight
	}

	return out
}

func FilterRouteSpecTLS(in interface{}) map[string]interface{} {
	tls, ok := in.(map[string]interface{})
	if !ok {
		return nil // Most likely a programming error.
	}

	out := make(map[string]interface{})
	if termination, ok := tls["termination"]; ok {
		out["termination"] = termination
	}

	if certificate, ok := tls["certificate"]; ok {
		out["certificate"] = certificate
	}

	if caCertificate, ok := tls["caCertificate"]; ok {
		out["caCertificate"] = caCertificate
	}

	if destinationCACertificate, ok := tls["destinationCACertificate"]; ok {
		out["destinationCACertificate"] = destinationCACertificate
	}

	if insecureEdgeTerminationPolicy, ok := tls["insecureEdgeTerminationPolicy"]; ok {
		out["insecureEdgeTerminationPolicy"] = insecureEdgeTerminationPolicy
	}

	return out
}
