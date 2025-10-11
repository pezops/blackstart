// Package v1alpha1
// +groupName=blackstart.pezops.github.io
package v1alpha1

import (
	"bytes"
	"encoding/json"

	"gopkg.in/yaml.v3"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workflow defines all the settings for a Blackstart workflow including its operations and their
// dependencies.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=workflows,scope=Namespaced,shortName=bswf
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Successful",type=string,JSONPath=".status.successful"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Operations",type=string,JSONPath=`.status.operationsCompleted`
// +kubebuilder:printcolumn:name="Last Ran",type=date,JSONPath=`.status.lastRan`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Workflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkflowSpec   `json:"spec,omitempty"`
	Status WorkflowStatus `json:"status,omitempty"`
}

// WorkflowConfigFile models a workflow as defined in a standalone YAML configuration file.
type WorkflowConfigFile struct {
	WorkflowSpec `yaml:",inline"`
	Name         string `yaml:"name" json:"name"`
}

// WorkflowList contains a list of Workflow resources
// +kubebuilder:object:root=true
type WorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workflow `json:"items"`
}

// WorkflowSpec models the spec section of the Workflow, the actual values used by Blackstart.
// +kubebuilder:object:generate=true
type WorkflowSpec struct {
	// Optional human description
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// A partially ordered set of operations to be executed.
	// +kubebuilder:validation:MinItems=1
	Operations []Operation `yaml:"operations" json:"operations"`
}

// Operation models a single Blackstart operation in the Workflow.
// +kubebuilder:object:generate=true
type Operation struct {
	// Short name for the operation.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Long-form description of the operation.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Identifier for the Operation, used by other operations to reference for dependencies.
	// +kubebuilder:validation:Required
	Id string `yaml:"id" json:"id"`

	// Module to be instantiated for the Operation. This must match the identifier of a registered
	// module.
	// +kubebuilder:validation:Required
	Module string `yaml:"module" json:"module"`

	// Inputs may be a key:value object mapping a set of static, scalar values to a named input
	// for the selected module. Instead of a scalar value, it may also be a well-known object with
	// the `fromDependency` property. The `fromDependency` must contain both an `id` and `output`
	// property to indicate which operation and output value to use as a dynamic input value that
	// is filled at runtime.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Inputs map[string]*OperationInput `yaml:"inputs,omitempty" json:"inputs,omitempty"`

	// DependsOn is a list of operation IDs that this operation depends on and must be completed
	// before this operation is run.
	DependsOn []string `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`

	// DoesNotExist is a special parameter that can be used to indicate that the resource should
	// not exist. This is useful for resources that are changed from a previous state and now
	// should be deleted if they still exist.
	DoesNotExist bool `yaml:"doesNotExist,omitempty" json:"doesNotExist,omitempty"`

	// Tainted is a special parameter that can be used to indicate that the resource is tainted and
	// should be replaced. This is useful for resources that always must be updated so that
	// attributes / output values are known by blackstart. This should not be configured by users,
	// and should only be used explicitly by modules.
	Tainted bool `yaml:"tainted,omitempty" json:"tainted,omitempty"`
}

// OperationInput is a single input value for an operation. Inputs may either be static or dynamic (from a
// dependency) using the output of a different operation.
// +kubebuilder:object:generate=true
type OperationInput struct {
	// FromDependency indicates that the input value should be taken from the output of a different
	// operation.
	FromDependency *FromDependency `yaml:"fromDependency,omitempty" json:"fromDependency,omitempty"`

	// Extra holds any additional fields not explicitly modeled in the struct. This should be a
	// map of scalar values.
	Extra *apiextensionsv1.JSON `yaml:"-" json:"-"`
}

// UnmarshalYAML implements custom YAML unmarshalling for OperationInput
func (oi *OperationInput) UnmarshalYAML(value *yaml.Node) error {
	var err error
	var raw map[string]interface{}
	if err = value.Decode(&raw); err == nil {
		if fd, ok := raw["fromDependency"]; ok {
			var buf []byte
			buf, err = yaml.Marshal(fd)
			if err != nil {
				return err
			}
			var dep FromDependency
			if err = yaml.Unmarshal(buf, &dep); err != nil {
				return err
			}
			oi.FromDependency = &dep
			delete(raw, "fromDependency")
		}
	}

	// Marshal the rest to JSON for the Extra field
	var remainingData interface{}
	if len(raw) > 0 {
		remainingData = raw
	} else if value.Kind != yaml.MappingNode {
		var simpleValue interface{}
		if err = value.Decode(&simpleValue); err != nil {
			return err
		}
		remainingData = simpleValue
	}

	if remainingData != nil {
		var jsonBytes []byte
		jsonBytes, err = yaml.Marshal(remainingData) // Use yaml.Marshal to handle the structure
		if err != nil {
			return err
		}
		// Remove the YAML document trailing new line if present
		jsonBytes = bytes.TrimSuffix(jsonBytes, []byte("\n"))
		oi.Extra = &apiextensionsv1.JSON{Raw: jsonBytes}
	}

	return nil
}

// UnmarshalJSON implements custom JSON unmarshalling for OperationInput
func (oi *OperationInput) UnmarshalJSON(data []byte) error {
	var err error
	var raw map[string]interface{}
	if err = json.Unmarshal(data, &raw); err == nil {
		if fd, ok := raw["fromDependency"]; ok {
			var buf []byte
			buf, err = json.Marshal(fd)
			if err != nil {
				return err
			}
			var dep FromDependency
			if err = json.Unmarshal(buf, &dep); err != nil {
				return err
			}
			oi.FromDependency = &dep
			delete(raw, "fromDependency")
		}
	}

	// Marshal the rest to JSON for the Extra field
	if len(raw) > 0 {
		var jsonBytes []byte
		jsonBytes, err = json.Marshal(raw)
		if err != nil {
			return err
		}
		oi.Extra = &apiextensionsv1.JSON{Raw: jsonBytes}
	} else {
		var simpleValue interface{}
		if err = json.Unmarshal(data, &simpleValue); err != nil {
			return err
		}
		jsonBytes, err := json.Marshal(simpleValue)
		if err != nil {
			return err
		}
		oi.Extra = &apiextensionsv1.JSON{Raw: jsonBytes}
	}

	return nil
}

// FromDependency models a dynamic input from the output of a different operation.
// +kubebuilder:object:generate=true
type FromDependency struct {
	// Id is the identifier of the operation to get the output value from.
	// +kubebuilder:validation:Required
	Id string `yaml:"id" json:"id"`

	// Output is the key used for the output value from a previously-ran dependency operation to
	// use as the input value for the current operation. This may include non-scalar values.
	// +kubebuilder:validation:Required
	Output string `yaml:"output" json:"output"`
}

// WorkflowStatus contains runtime status and result information about the Workflow.
// +kubebuilder:object:generate=true
type WorkflowStatus struct {
	// LastRan is the time the Workflow was last run, if ever.
	LastRan metav1.Time `json:"lastRan,omitempty"`

	// Successful indicates whether the last run was successful.
	Successful string `json:"successful,omitempty"`

	// Phase is a high-level state of the workflow that the last run ended in.
	Phase string `json:"phase,omitempty"`

	// Result contains any result information from the last run, including error messages if
	// applicable.
	Result string `json:"result,omitempty"`

	// OperationsCompleted is the number of operations that were completed in the last run. This
	// is stored in a fraction format where the denominator is the total number of operations in
	// the Workflow.
	OperationsCompleted string `json:"operationsCompleted,omitempty"`

	// LastOperation is the identifier of the last operation that was executed in the last run.
	LastOperation string `json:"lastOperation,omitempty"`
}
