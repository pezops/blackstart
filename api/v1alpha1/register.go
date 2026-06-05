package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const GroupName = "blackstart.pezops.github.io"
const GroupVersion = "v1alpha1"

var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
)

// addKnownTypes registers the API types for this group and version.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &Workflow{}, &WorkflowList{})
	return nil
}

func init() {
	utilruntime.Must(AddToScheme(runtime.NewScheme()))
}
