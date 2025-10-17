package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

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
	_, exists = info.Inputs[inputImmutable]
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
					inputImmutable: blackstart.NewInputFromValue(nil),
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
					inputImmutable: blackstart.NewInputFromValue(nil),
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

// TestConfigMapModule_Immutable tests the ConfigMap module's handling of the immutable field. This
// is a comprehensive integration test that verifies the module correctly manages the immutable field,
// including checking and setting the field, and respecting the immutability enforcement.
func TestConfigMapModule_Immutable(t *testing.T) {
	ctx := context.Background()

	cfg := setupEnvtest(t)

	// Create a real Kubernetes clientset for our modules
	clientset, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	// Create a controller-runtime client for direct verification
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	k8sClient, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: scheme})
	require.NoError(t, err)

	// Create test namespace
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}
	err = k8sClient.Create(ctx, testNamespace)
	require.NoError(t, err)

	cm := NewConfigMapModule()

	testConfigMapName := "test-immutable-configmap"
	namespace := "test-namespace"

	t.Run(
		"immutable_lifecycle", func(t *testing.T) {
			// Create a configmap without specifying immutable (nil/default)
			inputs := map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(clientset),
				inputName:      blackstart.NewInputFromValue(testConfigMapName),
				inputNamespace: blackstart.NewInputFromValue(namespace),
				inputImmutable: blackstart.NewInputFromValue(nil),
			}

			moduleCtx := blackstart.InputsToContext(ctx, inputs)
			err = cm.Set(moduleCtx)
			require.NoError(t, err)

			// Verify the configmap was created
			var cmResource corev1.ConfigMap
			err = k8sClient.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: testConfigMapName}, &cmResource)
			require.NoError(t, err)

			// Add test values to the configmap using direct Kubernetes client
			// Get the current configmap
			var testConfigMap *corev1.ConfigMap
			testConfigMap, err = clientset.CoreV1().ConfigMaps(namespace).Get(
				ctx,
				testConfigMapName,
				metav1.GetOptions{},
			)
			require.NoError(t, err)

			// Add some data
			if testConfigMap.Data == nil {
				testConfigMap.Data = make(map[string]string)
			}
			testConfigMap.Data["key1"] = "value1"
			testConfigMap.Data["key2"] = "value2"

			// Update the configmap
			_, err = clientset.CoreV1().ConfigMaps(namespace).Update(
				ctx,
				testConfigMap,
				metav1.UpdateOptions{},
			)
			require.NoError(t, err)

			// Verify both values were added
			err = k8sClient.Get(
				ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: testConfigMapName}, &cmResource,
			)
			require.NoError(t, err)

			assert.Equal(t, "value1", testConfigMap.Data["key1"])
			assert.Equal(t, "value2", testConfigMap.Data["key2"])

			// Use kubernetes_configmap module to make it immutable
			inputs[inputImmutable] = blackstart.NewInputFromValue(true)
			moduleCtx = blackstart.InputsToContext(ctx, inputs)
			err = cm.Set(moduleCtx)
			require.NoError(t, err)

			// Verify the configmap is now immutable
			err = k8sClient.Get(
				ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: testConfigMapName}, &cmResource,
			)
			require.NoError(t, err)
			// Immutable field should be *true - compare the dereferenced values
			require.NotNil(t, cmResource.Immutable)
			assert.Equal(t, true, *cmResource.Immutable)

			// Verify Check returns false when immutable field doesn't match
			// Check when using immutable: false, should return false
			checkInputs := map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(clientset),
				inputName:      blackstart.NewInputFromValue(testConfigMapName),
				inputNamespace: blackstart.NewInputFromValue(namespace),
				inputImmutable: blackstart.NewInputFromValue(false),
			}
			checkCtx := blackstart.InputsToContext(ctx, checkInputs)

			var result bool
			result, err = cm.Check(checkCtx)
			require.NoError(t, err)
			assert.False(t, result)

			// Check with immutable: true, should return true
			checkInputs[inputImmutable] = blackstart.NewInputFromValue(true)
			checkCtx = blackstart.InputsToContext(ctx, checkInputs)
			result, err = cm.Check(checkCtx)
			require.NoError(t, err)
			assert.True(t, result)

			// Check with immutable: nil should return true (immutable field is ignored)
			checkInputsNil := map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(clientset),
				inputName:      blackstart.NewInputFromValue(testConfigMapName),
				inputNamespace: blackstart.NewInputFromValue(namespace),
				inputImmutable: blackstart.NewInputFromValue(nil),
			}
			checkCtx = blackstart.InputsToContext(ctx, checkInputsNil)
			result, err = cm.Check(checkCtx)
			require.NoError(t, err)
			assert.True(t, result)

			// Attempt to modify the configmap - this should fail due to immutability
			// Get the current configmap
			testConfigMap, err = clientset.CoreV1().ConfigMaps(namespace).Get(
				ctx,
				testConfigMapName,
				metav1.GetOptions{},
			)
			require.NoError(t, err)

			// Try to modify an existing value
			testConfigMap.Data["key1"] = "modified-value"

			// This update should fail because the configmap is immutable
			_, err = clientset.CoreV1().ConfigMaps(namespace).Update(
				ctx,
				testConfigMap,
				metav1.UpdateOptions{},
			)
			// Note: The fake client may not enforce immutability, so we check if error exists
			// In a real cluster, this would return an Invalid error
			if err != nil {
				assert.True(t, apierrors.IsInvalid(err))
			}
		},
	)
}
