package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pezops/blackstart"
)

func TestConfigMapModule_Info(t *testing.T) {
	module := NewConfigMapModule()
	info := module.Info()

	assert.Equal(t, "kubernetes_configmap", info.Id)

	// Check required inputs
	_, exists := info.Inputs[inputName]
	assert.True(t, exists)
	_, exists = info.Inputs[inputNamespace]
	assert.True(t, exists)
	_, exists = info.Inputs[inputClient]
	assert.True(t, exists)

	// Check outputs
	_, exists = info.Outputs[outputConfigMap]
	assert.True(t, exists)
}

func TestConfigMapModule_Validate(t *testing.T) {
	module := NewConfigMapModule()
	fakeClientset := fake.NewClientset()

	tests := []struct {
		name        string
		inputs      map[string]blackstart.Input
		expectError bool
	}{
		{
			name: "valid inputs",
			inputs: map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(fakeClientset),
				inputName:      blackstart.NewInputFromValue("test-configmap"),
				inputNamespace: blackstart.NewInputFromValue("test-namespace"),
			},
			expectError: false,
		},
		{
			name: "missing client",
			inputs: map[string]blackstart.Input{
				inputName:      blackstart.NewInputFromValue("test-configmap"),
				inputNamespace: blackstart.NewInputFromValue("test-namespace"),
			},
			expectError: true,
		},
		{
			name: "missing name",
			inputs: map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(fakeClientset),
				inputNamespace: blackstart.NewInputFromValue("test-namespace"),
			},
			expectError: true,
		},
		{
			name: "empty string name",
			inputs: map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(fakeClientset),
				inputName:      blackstart.NewInputFromValue(""),
				inputNamespace: blackstart.NewInputFromValue("test-namespace"),
			},
			expectError: true,
		},
		{
			name: "missing namespace should use default",
			inputs: map[string]blackstart.Input{
				inputClient: blackstart.NewInputFromValue(fakeClientset),
				inputName:   blackstart.NewInputFromValue("test-configmap"),
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				operation := blackstart.Operation{
					Module: "kubernetes_configmap",
					Id:     "test",
					Inputs: test.inputs,
				}

				err := module.Validate(operation)
				if test.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			},
		)
	}
}

func TestConfigMapModule_Check(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewClientset()

	// Create test ConfigMap
	initialConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"key1": "value1",
		},
	}
	_, err := clientset.CoreV1().ConfigMaps("test-namespace").Create(
		context.Background(),
		initialConfigMap,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	module := NewConfigMapModule()

	tests := []struct {
		name           string
		configMapName  string
		namespace      string
		doesNotExist   bool
		tainted        bool
		expectedResult bool
		expectError    bool
	}{
		{
			name:           "existing configmap",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			expectedResult: true,
		},
		{
			name:           "missing configmap",
			configMapName:  "missing",
			namespace:      "test-namespace",
			expectedResult: false,
			expectError:    true,
		},
		{
			name:           "does not exist missing configmap",
			configMapName:  "missing",
			namespace:      "test-namespace",
			doesNotExist:   true,
			expectedResult: true,
		},
		{
			name:           "does not exist existing configmap",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			doesNotExist:   true,
			expectedResult: false,
		},
		{
			name:           "tainted existing configmap",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			tainted:        true,
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				// Create inputs
				inputs := map[string]blackstart.Input{
					inputClient:    blackstart.NewInputFromValue(clientset),
					inputName:      blackstart.NewInputFromValue(test.configMapName),
					inputNamespace: blackstart.NewInputFromValue(test.namespace),
				}

				// Create context using blackstart.InputsToContext
				ctx := context.Background()
				var flags []blackstart.ModuleContextFlag
				if test.doesNotExist {
					flags = append(flags, blackstart.DoesNotExistFlag)
				}
				if test.tainted {
					flags = append(flags, blackstart.TaintedFlag)
				}
				moduleContext := blackstart.InputsToContext(ctx, inputs, flags...)

				// Check
				var result bool
				result, err = module.Check(moduleContext)
				if test.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				assert.Equal(t, test.expectedResult, result)
			},
		)
	}
}

func TestConfigMapModule_Set(t *testing.T) {
	clientset := fake.NewClientset()

	initialConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"key1": "value1",
		},
	}
	_, err := clientset.CoreV1().ConfigMaps("test-namespace").Create(
		context.Background(),
		initialConfigMap,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	module := NewConfigMapModule()

	tests := []struct {
		name          string
		configMapName string
		namespace     string
		doesNotExist  bool
		checkAfter    func(t *testing.T, clientset *fake.Clientset)
	}{
		{
			name:          "create configmap",
			configMapName: "new-configmap",
			namespace:     "test-namespace",
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var testConfigMap *corev1.ConfigMap
				var tErr error
				testConfigMap, tErr = clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"new-configmap",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "new-configmap", testConfigMap.Name)
			},
		},
		{
			name:          "existing configmap",
			configMapName: "test-configmap",
			namespace:     "test-namespace",
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var testConfigMap *corev1.ConfigMap
				var tErr error
				testConfigMap, tErr = clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"test-configmap",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "test-configmap", testConfigMap.Name)
			},
		},
		{
			name:          "does not exist delete configmap",
			configMapName: "test-configmap",
			namespace:     "test-namespace",
			doesNotExist:  true,
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var tErr error
				_, tErr = clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"test-configmap",
					metav1.GetOptions{},
				)
				require.True(t, apierrors.IsNotFound(tErr))
			},
		},
		{
			name:          "does not exist missing configmap",
			configMapName: "missing-configmap",
			namespace:     "test-namespace",
			doesNotExist:  true,
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var tErr error
				_, tErr = clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"missing-configmap",
					metav1.GetOptions{},
				)
				assert.True(t, apierrors.IsNotFound(tErr))
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				// Create inputs
				inputs := map[string]blackstart.Input{
					inputClient:    blackstart.NewInputFromValue(clientset),
					inputName:      blackstart.NewInputFromValue(test.configMapName),
					inputNamespace: blackstart.NewInputFromValue(test.namespace),
				}

				// Create module context using blackstart.InputsToContext
				ctx := context.Background()
				var flags []blackstart.ModuleContextFlag
				if test.doesNotExist {
					flags = append(flags, blackstart.DoesNotExistFlag)
				}
				moduleContext := blackstart.InputsToContext(ctx, inputs, flags...)

				tErr := module.Set(moduleContext)
				require.NoError(t, tErr)

				if test.checkAfter != nil {
					test.checkAfter(t, clientset)
				}
			},
		)
	}
}
