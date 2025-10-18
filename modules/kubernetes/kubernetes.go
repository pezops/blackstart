package kubernetes

import "github.com/pezops/blackstart/util"

const (
	inputName         = "name"
	inputNamespace    = "namespace"
	inputKey          = "key"
	inputValue        = "value"
	inputClient       = "client"
	inputConfigMap    = "configmap"
	inputSecret       = "secret"
	inputImmutable    = "immutable"
	inputType         = "type"
	inputContext      = "context"
	inputUpdatePolicy = "update_policy"

	outputConfigMap = "configmap"
	outputSecret    = "secret"
	outputClient    = "client"
)

const (
	updatePolicyOverwrite   = "overwrite"
	updatePolicyPreserve    = "preserve"
	updatePolicyPreserveAny = "preserve_any"
	updatePolicyFail        = "fail"
)

var updatePolicies = map[string]string{
	updatePolicyOverwrite:   updatePolicyOverwrite,
	updatePolicyPreserve:    updatePolicyPreserve,
	updatePolicyPreserveAny: updatePolicyPreserveAny,
	updatePolicyFail:        updatePolicyFail,
}

var updatePolicyDocs = util.CleanString(
	`
**Update Policies**

Update policies control how existing values are handled when setting key-value pairs in ConfigMaps 
and Secrets. The following update policies are supported:

- '''overwrite''' - Existing values will be overwritten if they differ from the new value.
- '''preserve''' - Any non-empty, existing value will be preserved.
- '''preserve_any''' - Any existing value will be preserved.
- '''fail''' - If the new value differs from the existing value, the operation will fail.

`,
)
