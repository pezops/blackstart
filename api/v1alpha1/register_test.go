package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToSchemeRegistersListOptionsForParameterEncoding(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	require.NoError(t, err)

	codec := runtime.NewParameterCodec(scheme)
	_, err = codec.EncodeParameters(&metav1.ListOptions{}, SchemeGroupVersion)
	require.NoError(t, err)
}
