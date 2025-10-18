package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pezops/blackstart"
)

func TestConfigMapValueModule_Info(t *testing.T) {
	module := NewConfigMapValueModule()
	info := module.Info()

	assert.Equal(t, "kubernetes_configmap_value", info.Id)

	// Check required inputs
	_, exists := info.Inputs[inputKey]
	assert.True(t, exists)
	_, exists = info.Inputs[inputValue]
	assert.True(t, exists)
	_, exists = info.Inputs[inputConfigMap]
	assert.True(t, exists)
}

func TestConfigMapValueModule_Validate(t *testing.T) {
	module := NewConfigMapValueModule()
	fakeClientset := fake.NewClientset()
	namespace := "test-namespace"
	testCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: namespace,
		},
	}
	cm := &configMap{
		cm:  testCM,
		cmi: fakeClientset.CoreV1().ConfigMaps(namespace),
	}

	tests := []struct {
		name        string
		inputs      map[string]blackstart.Input
		expectError bool
	}{
		{
			name: "valid inputs",
			inputs: map[string]blackstart.Input{
				inputConfigMap:    blackstart.NewInputFromValue(cm),
				inputKey:          blackstart.NewInputFromValue("test-key"),
				inputValue:        blackstart.NewInputFromValue("test-value"),
				inputUpdatePolicy: blackstart.NewInputFromValue(updatePolicyOverwrite),
			},
			expectError: false,
		},
		{
			name: "missing configmap",
			inputs: map[string]blackstart.Input{
				inputKey:          blackstart.NewInputFromValue("test-key"),
				inputValue:        blackstart.NewInputFromValue("test-value"),
				inputUpdatePolicy: blackstart.NewInputFromValue(updatePolicyOverwrite),
			},
			expectError: true,
		},
		{
			name: "missing key",
			inputs: map[string]blackstart.Input{
				inputConfigMap:    blackstart.NewInputFromValue(cm),
				inputValue:        blackstart.NewInputFromValue("test-value"),
				inputUpdatePolicy: blackstart.NewInputFromValue(updatePolicyOverwrite),
			},
			expectError: true,
		},
		{
			name: "missing value",
			inputs: map[string]blackstart.Input{
				inputConfigMap:    blackstart.NewInputFromValue(cm),
				inputKey:          blackstart.NewInputFromValue("test-key"),
				inputUpdatePolicy: blackstart.NewInputFromValue(updatePolicyOverwrite),
			},
			expectError: true,
		},
		{
			name: "empty string key",
			inputs: map[string]blackstart.Input{
				inputConfigMap:    blackstart.NewInputFromValue(cm),
				inputKey:          blackstart.NewInputFromValue(""),
				inputValue:        blackstart.NewInputFromValue("test-value"),
				inputUpdatePolicy: blackstart.NewInputFromValue(updatePolicyOverwrite),
			},
			expectError: true,
		},
		{
			name: "empty string value is valid",
			inputs: map[string]blackstart.Input{
				inputConfigMap:    blackstart.NewInputFromValue(cm),
				inputKey:          blackstart.NewInputFromValue("test-key"),
				inputValue:        blackstart.NewInputFromValue(""),
				inputUpdatePolicy: blackstart.NewInputFromValue(updatePolicyOverwrite),
			},
			expectError: false,
		},
		{
			name: "invalid update policy",
			inputs: map[string]blackstart.Input{
				inputConfigMap:    blackstart.NewInputFromValue(cm),
				inputKey:          blackstart.NewInputFromValue("test-key"),
				inputValue:        blackstart.NewInputFromValue("test-value"),
				inputUpdatePolicy: blackstart.NewInputFromValue("invalid_policy"),
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				operation := blackstart.Operation{
					Module: "kubernetes_configmap_value",
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

func TestConfigMapValueModule_Check(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewClientset()

	// Create test ConfigMap
	initialConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"existing-key": "existing-value",
			"empty-key":    "",
		},
	}
	_, err := clientset.CoreV1().ConfigMaps("test-namespace").Create(
		context.Background(),
		initialConfigMap,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	module := NewConfigMapValueModule()

	tests := []struct {
		name           string
		configMapName  string
		namespace      string
		key            string
		value          string
		updatePolicy   string
		doesNotExist   bool
		tainted        bool
		expectedResult bool
		expectError    bool
	}{
		// Basic overwrite policy tests
		{
			name:           "existing configmap missing key - overwrite",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "new-key",
			value:          "new-value",
			updatePolicy:   updatePolicyOverwrite,
			expectedResult: false,
		},
		{
			name:           "existing configmap existing key incorrect value - overwrite",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "different-value",
			updatePolicy:   updatePolicyOverwrite,
			expectedResult: false,
		},
		{
			name:           "existing configmap existing key correct value - overwrite",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "existing-value",
			updatePolicy:   updatePolicyOverwrite,
			expectedResult: true,
		},
		// Preserve policy tests
		{
			name:           "preserve policy with non-empty existing value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "different-value",
			updatePolicy:   updatePolicyPreserve,
			expectedResult: true,
		},
		{
			name:           "preserve policy with empty existing value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "empty-key",
			value:          "new-value",
			updatePolicy:   updatePolicyPreserve,
			expectedResult: false,
		},
		{
			name:           "preserve policy with missing key",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "missing-key",
			value:          "new-value",
			updatePolicy:   updatePolicyPreserve,
			expectedResult: false,
		},
		// Preserve_any policy tests
		{
			name:           "preserve_any policy with non-empty existing value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "different-value",
			updatePolicy:   updatePolicyPreserveAny,
			expectedResult: true,
		},
		{
			name:           "preserve_any policy with empty existing value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "empty-key",
			value:          "new-value",
			updatePolicy:   updatePolicyPreserveAny,
			expectedResult: true,
		},
		{
			name:           "preserve_any policy with missing key",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "missing-key",
			value:          "new-value",
			updatePolicy:   updatePolicyPreserveAny,
			expectedResult: false,
		},
		// Fail policy tests
		{
			name:           "fail policy with matching value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "existing-value",
			updatePolicy:   updatePolicyFail,
			expectedResult: true,
		},
		{
			name:           "fail policy with different value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "different-value",
			updatePolicy:   updatePolicyFail,
			expectedResult: false,
			expectError:    true,
		},
		{
			name:           "fail policy with missing key",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "missing-key",
			value:          "new-value",
			updatePolicy:   updatePolicyFail,
			expectedResult: false,
		},
		// Other test cases
		{
			name:           "missing configmap",
			configMapName:  "missing",
			namespace:      "test-namespace",
			key:            "some-key",
			value:          "some-value",
			updatePolicy:   updatePolicyOverwrite,
			expectedResult: false,
		},
		{
			name:           "does not exist missing key",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "missing-key",
			value:          "any-value",
			updatePolicy:   updatePolicyOverwrite,
			doesNotExist:   true,
			expectedResult: true,
		},
		{
			name:           "does not exist existing key",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "any-value",
			updatePolicy:   updatePolicyOverwrite,
			doesNotExist:   true,
			expectedResult: false,
		},
		{
			name:           "tainted existing configmap correct value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "existing-value",
			updatePolicy:   updatePolicyOverwrite,
			tainted:        true,
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				// Get the ConfigMap and create a configMap object
				var cm *corev1.ConfigMap
				var tErr error
				if test.configMapName == "test-configmap" {
					cm, tErr = clientset.CoreV1().ConfigMaps(test.namespace).Get(
						context.Background(),
						test.configMapName,
						metav1.GetOptions{},
					)
					require.NoError(t, tErr)
				} else {
					cm = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      test.configMapName,
							Namespace: test.namespace,
						},
					}
				}

				configMapObj := &configMap{
					cm:  cm,
					cmi: clientset.CoreV1().ConfigMaps(test.namespace),
				}

				// Create inputs
				inputs := map[string]blackstart.Input{
					inputConfigMap:    blackstart.NewInputFromValue(configMapObj),
					inputKey:          blackstart.NewInputFromValue(test.key),
					inputValue:        blackstart.NewInputFromValue(test.value),
					inputUpdatePolicy: blackstart.NewInputFromValue(test.updatePolicy),
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

				// Call Check method
				result, tErr := module.Check(moduleContext)

				if test.expectError {
					require.Error(t, tErr)
				} else {
					require.NoError(t, tErr)
				}

				// Verify result
				assert.Equal(t, test.expectedResult, result)
			},
		)
	}
}

func TestConfigMapValueModule_Set(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewClientset()

	// Create test ConfigMap
	initialConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"existing-key": "existing-value",
		},
	}
	_, err := clientset.CoreV1().ConfigMaps("test-namespace").Create(
		context.Background(),
		initialConfigMap,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	module := NewConfigMapValueModule()

	tests := []struct {
		name          string
		configMapName string
		namespace     string
		key           string
		value         string
		doesNotExist  bool
		checkAfter    func(t *testing.T, clientset *fake.Clientset)
	}{
		{
			name:          "update existing ConfigMap with new key",
			configMapName: "test-configmap",
			namespace:     "test-namespace",
			key:           "new-key",
			value:         "new-value",
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				cm, tErr := clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"test-configmap",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "new-value", cm.Data["new-key"])
				assert.Equal(t, "existing-value", cm.Data["existing-key"])
			},
		},
		{
			name:          "update existing ConfigMap with existing key",
			configMapName: "test-configmap",
			namespace:     "test-namespace",
			key:           "existing-key",
			value:         "updated-value",
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				cm, tErr := clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"test-configmap",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "updated-value", cm.Data["existing-key"])
			},
		},
		{
			name:          "does not exist existing key",
			configMapName: "test-configmap",
			namespace:     "test-namespace",
			key:           "existing-key",
			value:         "any-value",
			doesNotExist:  true,
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				cm, tErr := clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"test-configmap",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				_, exists := cm.Data["existing-key"]
				assert.False(t, exists)
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				// Get the ConfigMap if it already exists, or create a new one
				var cm *corev1.ConfigMap
				var tEerr error

				if test.configMapName == "test-configmap" {
					cm, tEerr = clientset.CoreV1().ConfigMaps(test.namespace).Get(
						context.Background(),
						test.configMapName,
						metav1.GetOptions{},
					)
					require.NoError(t, tEerr)
				} else {
					// For new ConfigMap, create a new object (but don't add to clientset yet)
					cm = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      test.configMapName,
							Namespace: test.namespace,
						},
					}
				}

				configMapObj := &configMap{
					cm:  cm,
					cmi: clientset.CoreV1().ConfigMaps(test.namespace),
				}

				// Create inputs
				inputs := map[string]blackstart.Input{
					inputConfigMap: blackstart.NewInputFromValue(configMapObj),
					inputKey:       blackstart.NewInputFromValue(test.key),
					inputValue:     blackstart.NewInputFromValue(test.value),
				}

				// Create context using blackstart.InputsToContext
				ctx := context.Background()
				var flags []blackstart.ModuleContextFlag
				if test.doesNotExist {
					flags = append(flags, blackstart.DoesNotExistFlag)
				}
				moduleContext := blackstart.InputsToContext(ctx, inputs, flags...)

				// Call Set method
				tEerr = module.Set(moduleContext)
				require.NoError(t, tEerr)

				// Run verification
				if test.checkAfter != nil {
					test.checkAfter(t, clientset)
				}
			},
		)
	}
}

func TestConfigMapValueModule(t *testing.T) {
	clientset := fake.NewClientset()

	module := NewConfigMapValueModule()

	testNamespace := "test-namespace"
	testConfigMapName := "test-configmap"
	testKey := "test-key"
	testValue := "test-value"

	t.Run(
		"configmap lifecycle", func(t *testing.T) {
			// Check for positive and negative does not exist variations of a missing configmap

			nonExistentK8sConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testConfigMapName,
					Namespace: testNamespace,
				},
			}

			nonExistentConfigMap := &configMap{
				cm:  nonExistentK8sConfigMap,
				cmi: clientset.CoreV1().ConfigMaps(testNamespace),
			}

			// Create inputs for testing missing ConfigMap
			inputs := map[string]blackstart.Input{
				inputConfigMap:    blackstart.NewInputFromValue(nonExistentConfigMap),
				inputKey:          blackstart.NewInputFromValue(testKey),
				inputValue:        blackstart.NewInputFromValue(testValue),
				inputUpdatePolicy: blackstart.NewInputFromValue(updatePolicyOverwrite),
			}

			// Check for a missing ConfigMap
			ctx := blackstart.InputsToContext(context.Background(), inputs)
			result, err := module.Check(ctx)
			require.NoError(t, err)
			assert.False(t, result)

			// Check for a missing ConfigMap with "does not exist" set
			ctx = blackstart.InputsToContext(context.Background(), inputs, blackstart.DoesNotExistFlag)
			result, err = module.Check(ctx)
			require.NoError(t, err)
			assert.True(t, result)

			// Create a new test ConfigMap
			_, err = clientset.CoreV1().ConfigMaps(testNamespace).Create(
				context.Background(),
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testConfigMapName,
						Namespace: testNamespace,
					},
					Data: map[string]string{},
				},
				metav1.CreateOptions{},
			)
			require.NoError(t, err)

			// Get the ConfigMap
			cm, err := clientset.CoreV1().ConfigMaps(testNamespace).Get(
				context.Background(),
				testConfigMapName,
				metav1.GetOptions{},
			)
			require.NoError(t, err)

			// Create a configMap object with the real ConfigMap
			configMapObj := &configMap{
				cm:  cm,
				cmi: clientset.CoreV1().ConfigMaps(testNamespace),
			}

			// Update inputs with the real ConfigMap
			inputs[inputConfigMap] = blackstart.NewInputFromValue(configMapObj)

			// Check that the key doesn't exist
			ctx = blackstart.InputsToContext(context.Background(), inputs)
			result, err = module.Check(ctx)
			require.NoError(t, err)
			assert.False(t, result)

			// Set the value
			err = module.Set(ctx)
			require.NoError(t, err)

			// Verify the value was set
			cm, err = clientset.CoreV1().ConfigMaps(testNamespace).Get(
				context.Background(),
				testConfigMapName,
				metav1.GetOptions{},
			)
			require.NoError(t, err)
			assert.Equal(t, testValue, cm.Data[testKey])

			// Update the configMapObj with the updated ConfigMap
			configMapObj = &configMap{
				cm:  cm,
				cmi: clientset.CoreV1().ConfigMaps(testNamespace),
			}
			inputs[inputConfigMap] = blackstart.NewInputFromValue(configMapObj)

			// Check that the value was set correctly
			ctx = blackstart.InputsToContext(context.Background(), inputs)
			result, err = module.Check(ctx)
			require.NoError(t, err)
			assert.True(t, result)

			// Delete the value using Set with DoesNotExist set
			ctx = blackstart.InputsToContext(context.Background(), inputs, blackstart.DoesNotExistFlag)
			err = module.Set(ctx)
			require.NoError(t, err)

			// Verify the key was deleted
			cm, err = clientset.CoreV1().ConfigMaps(testNamespace).Get(
				context.Background(),
				testConfigMapName,
				metav1.GetOptions{},
			)
			require.NoError(t, err)
			_, exists := cm.Data[testKey]
			assert.False(t, exists)

			// Update the configMapObj with the updated ConfigMap
			configMapObj = &configMap{
				cm:  cm,
				cmi: clientset.CoreV1().ConfigMaps(testNamespace),
			}
			inputs[inputConfigMap] = blackstart.NewInputFromValue(configMapObj)

			// Check that the key is gone
			ctx = blackstart.InputsToContext(context.Background(), inputs)
			result, err = module.Check(ctx)
			require.NoError(t, err)
			assert.False(t, result)

			// Verify it succeeds with DoesNotExist set
			ctx = blackstart.InputsToContext(context.Background(), inputs, blackstart.DoesNotExistFlag)
			result, err = module.Check(ctx)
			require.NoError(t, err)
			assert.True(t, result)
		},
	)
}
