package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const GroupName = "blackstart.pezops.github.io"
const GroupVersion = "v1alpha1"

var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	SchemeBuilder      = &scheme.Builder{GroupVersion: SchemeGroupVersion}
	AddToScheme        = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&Workflow{}, &WorkflowList{})
	utilruntime.Must(AddToScheme(runtime.NewScheme()))
}
