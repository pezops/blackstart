package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pezops/blackstart"
)

func TestConfigMapValueModule_Info(t *testing.T) {
	module := NewConfigMapValueModule()
	info := module.Info()

	if info.Id != "kubernetes_configmap_value" {
		t.Errorf("Expected ID to be 'kubernetes_configmap_value', got '%s'", info.Id)
	}

	// Check required inputs
	if _, exists := info.Inputs[inputKey]; !exists {
		t.Errorf("Expected input '%s' to exist", inputKey)
	}
	if _, exists := info.Inputs[inputValue]; !exists {
		t.Errorf("Expected input '%s' to exist", inputValue)
	}
	if _, exists := info.Inputs[inputConfigMap]; !exists {
		t.Errorf("Expected input '%s' to exist", inputConfigMap)
	}
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
				inputConfigMap: blackstart.NewInputFromValue(cm),
				inputKey:       blackstart.NewInputFromValue("test-key"),
				inputValue:     blackstart.NewInputFromValue("test-value"),
			},
			expectError: false,
		},
		{
			name: "missing configmap",
			inputs: map[string]blackstart.Input{
				inputKey:   blackstart.NewInputFromValue("test-key"),
				inputValue: blackstart.NewInputFromValue("test-value"),
			},
			expectError: true,
		},
		{
			name: "missing key",
			inputs: map[string]blackstart.Input{
				inputConfigMap: blackstart.NewInputFromValue(cm),
				inputValue:     blackstart.NewInputFromValue("test-value"),
			},
			expectError: true,
		},
		{
			name: "missing value",
			inputs: map[string]blackstart.Input{
				inputConfigMap: blackstart.NewInputFromValue(cm),
				inputKey:       blackstart.NewInputFromValue("test-key"),
			},
			expectError: true,
		},
		{
			name: "empty string key",
			inputs: map[string]blackstart.Input{
				inputConfigMap: blackstart.NewInputFromValue(cm),
				inputKey:       blackstart.NewInputFromValue(""),
				inputValue:     blackstart.NewInputFromValue("test-value"),
			},
			expectError: true,
		},
		{
			name: "empty string value is valid",
			inputs: map[string]blackstart.Input{
				inputConfigMap: blackstart.NewInputFromValue(cm),
				inputKey:       blackstart.NewInputFromValue("test-key"),
				inputValue:     blackstart.NewInputFromValue(""),
			},
			expectError: false,
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
				if test.expectError && err == nil {
					t.Error("Expected error but got none")
				}
				if !test.expectError && err != nil {
					t.Errorf("Expected no error but got: %v", err)
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
		},
	}
	_, err := clientset.CoreV1().ConfigMaps("test-namespace").Create(
		context.Background(),
		initialConfigMap,
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	module := NewConfigMapValueModule()

	tests := []struct {
		name           string
		configMapName  string
		namespace      string
		key            string
		value          string
		doesNotExist   bool
		tainted        bool
		expectedResult bool
	}{
		{
			name:           "existing ConfigMap, non-existing key",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "new-key",
			value:          "new-value",
			expectedResult: false,
		},
		{
			name:           "existing ConfigMap, existing key, wrong value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "different-value",
			expectedResult: false,
		},
		{
			name:           "existing ConfigMap, existing key, correct value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "existing-value",
			expectedResult: true,
		},
		{
			name:           "non-existing ConfigMap",
			configMapName:  "non-existing",
			namespace:      "test-namespace",
			key:            "some-key",
			value:          "some-value",
			expectedResult: false,
		},
		{
			name:           "does not exist mode, non-existing key",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "non-existing-key",
			value:          "any-value",
			doesNotExist:   true,
			expectedResult: true,
		},
		{
			name:           "does not exist mode, existing key",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "any-value",
			doesNotExist:   true,
			expectedResult: false,
		},
		{
			name:           "tainted mode, existing ConfigMap and correct value",
			configMapName:  "test-configmap",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "existing-value",
			tainted:        true,
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				// Get the ConfigMap and create a configMap object
				var cm *corev1.ConfigMap
				if test.configMapName == "test-configmap" {
					var err error
					cm, err = clientset.CoreV1().ConfigMaps(test.namespace).Get(
						context.Background(),
						test.configMapName,
						metav1.GetOptions{},
					)
					if err != nil {
						t.Fatalf("Failed to get ConfigMap: %v", err)
					}
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

				// Create inputs (removed inputClient since it's embedded in configMap now)
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
				if test.tainted {
					flags = append(flags, blackstart.TaintedFlag)
				}
				moduleContext := blackstart.InputsToContext(ctx, inputs, flags...)

				// Call Check method
				result, err := module.Check(moduleContext)
				if err != nil {
					t.Fatalf("Check failed with error: %v", err)
				}

				// Verify result
				if result != test.expectedResult {
					t.Errorf("Expected result to be %v, got %v", test.expectedResult, result)
				}
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
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

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
				cm, err := clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"test-configmap",
					metav1.GetOptions{},
				)
				if err != nil {
					t.Fatalf("Failed to get ConfigMap: %v", err)
				}
				if cm.Data["new-key"] != "new-value" {
					t.Errorf("Expected value to be 'new-value', got '%s'", cm.Data["new-key"])
				}
				if cm.Data["existing-key"] != "existing-value" {
					t.Errorf("Expected existing key to be unchanged")
				}
			},
		},
		{
			name:          "update existing ConfigMap with existing key",
			configMapName: "test-configmap",
			namespace:     "test-namespace",
			key:           "existing-key",
			value:         "updated-value",
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				cm, err := clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"test-configmap",
					metav1.GetOptions{},
				)
				if err != nil {
					t.Fatalf("Failed to get ConfigMap: %v", err)
				}
				if cm.Data["existing-key"] != "updated-value" {
					t.Errorf("Expected value to be 'updated-value', got '%s'", cm.Data["existing-key"])
				}
			},
		},
		{
			name:          "delete key in does not exist mode",
			configMapName: "test-configmap",
			namespace:     "test-namespace",
			key:           "existing-key",
			value:         "any-value",
			doesNotExist:  true,
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				cm, err := clientset.CoreV1().ConfigMaps("test-namespace").Get(
					context.Background(),
					"test-configmap",
					metav1.GetOptions{},
				)
				if err != nil {
					t.Fatalf("Failed to get ConfigMap: %v", err)
				}
				if _, exists := cm.Data["existing-key"]; exists {
					t.Errorf("Expected key to be deleted, but it still exists")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				// Get the ConfigMap if it already exists, or create a new one
				var cm *corev1.ConfigMap
				var err error

				if test.configMapName == "test-configmap" {
					cm, err = clientset.CoreV1().ConfigMaps(test.namespace).Get(
						context.Background(),
						test.configMapName,
						metav1.GetOptions{},
					)
					if err != nil {
						t.Fatalf("Failed to get ConfigMap: %v", err)
					}
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

				// Create inputs (removed inputClient since it's embedded in configMap now)
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
				err = module.Set(moduleContext)
				if err != nil {
					t.Fatalf("Set failed with error: %v", err)
				}

				// Run verification
				if test.checkAfter != nil {
					test.checkAfter(t, clientset)
				}
			},
		)
	}
}

func TestConfigMapValueModule(t *testing.T) {
	// 1. Set up a fake k8s client
	clientset := fake.NewClientset()

	// 2. Create the configmap_value module
	module := NewConfigMapValueModule()

	// Define test parameters
	testNamespace := "test-namespace"
	testConfigMapName := "test-configmap"
	testKey := "test-key"
	testValue := "test-value"

	t.Run(
		"ConfigMap lifecycle", func(t *testing.T) {
			// 3. Check for positive and negative does not exist variations of a missing configmap
			// First, test with a non-existent ConfigMap
			// Create a ConfigMap object for a non-existent ConfigMap
			nonExistentCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testConfigMapName,
					Namespace: testNamespace,
				},
			}

			nonExistentConfigMap := &configMap{
				cm:  nonExistentCM,
				cmi: clientset.CoreV1().ConfigMaps(testNamespace),
			}

			// Create inputs for testing non-existent ConfigMap (removed inputClient)
			inputs := map[string]blackstart.Input{
				inputConfigMap: blackstart.NewInputFromValue(nonExistentConfigMap),
				inputKey:       blackstart.NewInputFromValue(testKey),
				inputValue:     blackstart.NewInputFromValue(testValue),
			}

			// Check for a non-existent ConfigMap (regular mode)
			ctx := blackstart.InputsToContext(context.Background(), inputs)
			result, err := module.Check(ctx)
			if err != nil {
				t.Fatalf("Check failed with error: %v", err)
			}
			if result {
				t.Errorf("Expected Check to return false for non-existent ConfigMap, got true")
			}

			// Check for a non-existent ConfigMap in "does not exist" mode
			ctx = blackstart.InputsToContext(context.Background(), inputs, blackstart.DoesNotExistFlag)
			result, err = module.Check(ctx)
			if err != nil {
				t.Fatalf("Check failed with error: %v", err)
			}
			if !result {
				t.Errorf("Expected Check to return true for non-existent ConfigMap in DoesNotExist mode, got false")
			}

			// 4. Use the fake client to create a new test-configmap
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
			if err != nil {
				t.Fatalf("Failed to create test ConfigMap: %v", err)
			}

			// Verify the ConfigMap was created
			cm, err := clientset.CoreV1().ConfigMaps(testNamespace).Get(
				context.Background(),
				testConfigMapName,
				metav1.GetOptions{},
			)
			if err != nil {
				t.Fatalf("Failed to get ConfigMap: %v", err)
			}

			// Create a configMap object with the real ConfigMap
			configMapObj := &configMap{
				cm:  cm,
				cmi: clientset.CoreV1().ConfigMaps(testNamespace),
			}

			// Update inputs with the real ConfigMap
			inputs[inputConfigMap] = blackstart.NewInputFromValue(configMapObj)

			// 5. Use the module to set a value in the configmap, verify it's set
			// First check that the key doesn't exist
			ctx = blackstart.InputsToContext(context.Background(), inputs)
			result, err = module.Check(ctx)
			if err != nil {
				t.Fatalf("Check failed with error: %v", err)
			}
			if result {
				t.Errorf("Expected Check to return false for key that doesn't exist yet, got true")
			}

			// Set the value
			err = module.Set(ctx)
			if err != nil {
				t.Fatalf("Set failed with error: %v", err)
			}

			// Verify the value was set by retrieving the ConfigMap directly
			cm, err = clientset.CoreV1().ConfigMaps(testNamespace).Get(
				context.Background(),
				testConfigMapName,
				metav1.GetOptions{},
			)
			if err != nil {
				t.Fatalf("Failed to get ConfigMap: %v", err)
			}

			// Verify the key and value were set correctly
			if value, exists := cm.Data[testKey]; !exists {
				t.Errorf("Expected key %s to exist in ConfigMap, but it doesn't", testKey)
			} else if value != testValue {
				t.Errorf("Expected value to be %s, got %s", testValue, value)
			}

			// Update the configMapObj with the updated ConfigMap
			configMapObj = &configMap{
				cm:  cm,
				cmi: clientset.CoreV1().ConfigMaps(testNamespace),
			}
			inputs[inputConfigMap] = blackstart.NewInputFromValue(configMapObj)

			// Check that the value was set correctly using the module
			ctx = blackstart.InputsToContext(context.Background(), inputs)
			result, err = module.Check(ctx)
			if err != nil {
				t.Fatalf("Check failed with error: %v", err)
			}
			if !result {
				t.Errorf("Expected Check to return true for key that exists with correct value, got false")
			}

			// 6. Use the module to delete a value in the configmap, verify it's missing
			// Set DoesNotExist mode to delete the key
			ctx = blackstart.InputsToContext(context.Background(), inputs, blackstart.DoesNotExistFlag)

			// Delete the value using Set in DoesNotExist mode
			err = module.Set(ctx)
			if err != nil {
				t.Fatalf("Set (delete) failed with error: %v", err)
			}

			// Verify the key was deleted by retrieving the ConfigMap directly
			cm, err = clientset.CoreV1().ConfigMaps(testNamespace).Get(
				context.Background(),
				testConfigMapName,
				metav1.GetOptions{},
			)
			if err != nil {
				t.Fatalf("Failed to get ConfigMap: %v", err)
			}

			// Verify the key no longer exists
			if _, exists := cm.Data[testKey]; exists {
				t.Errorf("Expected key %s to be deleted from ConfigMap, but it still exists", testKey)
			}

			// Update the configMapObj with the updated ConfigMap
			configMapObj = &configMap{
				cm:  cm,
				cmi: clientset.CoreV1().ConfigMaps(testNamespace),
			}
			inputs[inputConfigMap] = blackstart.NewInputFromValue(configMapObj)

			// Check that the key is gone using the module
			ctx = blackstart.InputsToContext(context.Background(), inputs)
			result, err = module.Check(ctx)
			if err != nil {
				t.Fatalf("Check failed with error: %v", err)
			}
			if result {
				t.Errorf("Expected Check to return false for deleted key, got true")
			}

			// Verify it succeeds in DoesNotExist mode
			ctx = blackstart.InputsToContext(context.Background(), inputs, blackstart.DoesNotExistFlag)
			result, err = module.Check(ctx)
			if err != nil {
				t.Fatalf("Check failed with error: %v", err)
			}
			if !result {
				t.Errorf("Expected Check to return true for deleted key in DoesNotExist mode, got false")
			}
		},
	)
}
