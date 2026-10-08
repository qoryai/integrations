module github.com/qoryai/integrations

go 1.27.1

require (
	github.com/qoryai/runner v0.6.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	gopkg.in/yaml.v3 v3.0.1
)

require golang.org/x/text v0.14.0 // indirect

// The module proxy holds an earlier tree under v0.1.0, without the conformance package.
retract v0.1.0
