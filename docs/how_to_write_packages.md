# How to write Preflight Packages

## What is a Preflight package?

A Preflight package contains the definition of a policy. A policy is a set of rules that Preflight will check in your cluster.

Preflight packages are made of two well distinguished parts, the _policy manifest_ and the _Rego_ definition of the rules.

<img align="center" width="460" height="300" src="./images/preflight_package.png">

## Writing the _policy manifest_

The _policy manifest_ is a _YAML_ file that contains information about your policy. You can see [here](https://godoc.org/github.com/jetstack/preflight/pkg/packaging#PolicyManifest) the schema of this file.

There is some metadata for the package, such as the name and the description.

The rules of your policy must also be declared here. They are organized in sections. Every section can have a name, a description, and a list of rules.

Every rule has its own name, description, useful links, and remediation instructions. This guide has a dedicated section about how to add rules.

### Choose your _data-gatherers_

Preflight evaluates the policies against data that it fetches from different sources. These sources are the _data-gatherers_. You can see [here](./datagatherers) a list of the available _data-gatherers_ and their documentation. In the future, Preflight will support sourcing data from external _data-gatherers_ (#24).

The _data-gatherers_ your package requires should be declared in the _policy manifest_.

```yaml
schema-version: "1.0.0"
id: "mypackage"
namespace: "examples.jetstack.io"
package-version: "1.0.0"
data-gatherers:
- k8s/pods
- gke
...
```

### Versioning

Preflight packages are intended to evolve. Policies can be changed, new rules can be added or deleted, and all the metadata can mutate.

In order to ease keeping track of those changes, Preflight packages have a version tag. That version is specified with the `package-version` property in the _policy manifest_.

### The minimal _policy manifest_

Let's write a minimal _policy manifest_ to get started.

First, create a directory for this new package. We are going to create this new package under the `examples.jetstack.io` namespace, and we are going to name it `podsbestpractices`.

Then create the `policy-manifest.yaml` file. The following fields are mandatory:

- `schema-version`: indicates which schema is being used for the _policy manifest_. For now, there is only version: `1.0.0`.
- `namespace`, `id`, and `package-version`: these properties identify the package. `namespace` must be a FQDN and it is encouraged that `package-version` uses [semver](https://semver.org).
- `root-query`: Name of the Rego package containing the rules backing the
  package (see below).

Then, you should also declare the _data-gatherers_ that your rules are going to need. For this example, we only need one, `k8s/pods`.

Finally, it's time to declare the rules for the policy. Rules are organized into sections. Every section has an ID, a name, and a description. Also, every rule has its own ID, name, and description. Additionally, rules can have other metadata like a remediation advice or a set of related links.

For simplicity's sake, this example contains just one section with one rule.

```
# preflight-packages/examples.jetstack.io/podsbestpractices/policy-manifest.yaml

schema-version: "1.0.0"
id: "podsbestpractices"
namespace: "examples.jetstack.io"
package-version: "1.0.0"
root-query: "data.pods" # the concept of `root-query` is explained later in this doc
data-gatherers:
- k8s/pods
sections:
- id: images
  name: Images
  description: "Restrictions over the images."
  rules:
  - id: tag_not_latest
    name: "Tag is not latest"
    description: >
      Avoid using "latest" as tag for the image since.
    remediation: >
      Change your manifest and edit the Pod template so the image is pinned to a certain tag.
    links:
      - "https://kubernetes.io/docs/concepts/containers/images/"
```

## Writing the policy definition in Rego

In the previous section, we created the _policy manifest_, which contains a human readable description of the rules in our policy. Now it's time to define the same rules in a language that is machine readable.

### The Rego package

Preflight relies on Open Policy Agent as the policy engine. Rego is OPA's language to define policies. You can find their comprehensive [documentation here](https://www.openpolicyagent.org/docs/latest/policy-language/).

You can have multiple Rego files inside the directory of a Preflight package.  All the Rego rules corresponding to the _policy manifest_ rules must be in the same Rego package, and that package must be indicated in the _policy manifest_ using the `root-query` property.

For instance, this snippet shows an arbitrary Rego rule in a package named `podsbestpractices`:

```
package pods

import input["k8s/pods"] as pods

preflight_tag_not_latest[message] {
  true
  message := "true was found to be true"
}
```

As you can identify, the Rego package for that policy is `pods`. In this case, OPA's `root-query` is `data.pods`, and that is why in the previous section, `policy-manifest.yaml` contains `root-query: "data.pods"`.

### Writing Rego rules

Rego is a declarative language and has a bit of a learning curve. It is strongly recommended to read ["The Basics"](https://www.openpolicyagent.org/docs/latest/policy-language/#the-basics). Also, it is useful to have the [language reference](https://www.openpolicyagent.org/docs/latest/policy-reference/) to hand.

In order to speed up the process of writing Rego rules, it's best to write tests. It means you can iterate fast while writing rules and make sure the rules are doing what you intended. It is conventional to name the test files for `policy.rego` as `policy_test.rego`.

This example contains the definition for the `tag_no_latest` rule. As you can see, there is the convention within Preflight to add `preflight_` as prefix to the rule ID when that is written in Rego (related issue #27).

```
# preflight-packages/examples.jetstack.io/podsbestpractices/policy.rego

package pods

import input["k8s/pods"] as pods

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
```

### Testing Rego

As mentioned before, it is very useful to [write tests for your Rego rules](https://www.openpolicyagent.org/docs/latest/policy-testing/).

This snippet contains a test case for the previous Rego code.

```
# preflight-packages/examples.jetstack.io/podsbestpractices/policy_test.rego

package pods

pods(x) = y { y := {"k8s/pods": {"items": x }} }

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

test_explicit_image_tag_no_pods {
	output := preflight_explicit_image_tag with input as pods([])
	assert_allowed(output)
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

	assert_violates(output, {
		"container 'container-one' in pod 'foo' in namespace 'default' is missing an explicit image tag"
		})
}
```

Soon, Preflight will be able to run Rego tests inside Preflight packages (#26), but unfortunately this is not possible yet.

To run our tests, we must use the [OPA command line](https://www.openpolicyagent.org/docs/latest/#running-opa) like this:

```
opa test -v --explain=notes ./preflight-packages/examples.jetstack.io/podsbestpractices
```

## Lint your packages

The Preflight command line has a built-in linter. This helps to make sure that the package follows the best practices.

You can lint your package by running:

```
preflight package lint ./preflight-packages/examples.jetstack.io/podsbestpractices
```

## Configure Preflight to use your package

The last step would be to tell Preflight to actually use these new package. That configuration goes into the `preflight.yaml` file. For this example, it would look like this:

```yaml
# preflight.yaml
cluster-name: my-cluster

data-gatherers:
  k8s/pods:
    kubeconfig: ~/.kube/config

package-sources:
- type: local
  dir: preflight-packages/

enabled-packages:
  - "examples.jetstack.io/podsbestpractice"

outputs:
- type: local
  path: ./output
  format: json
- type: cli
```
