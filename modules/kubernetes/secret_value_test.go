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

func TestSecretValueModule_Info(t *testing.T) {
	module := NewSecretValueModule()
	info := module.Info()

	assert.Equal(t, "kubernetes_secret_value", info.Id)

	// Check required inputs
	_, exists := info.Inputs[inputKey]
	assert.True(t, exists)
	_, exists = info.Inputs[inputValue]
	assert.True(t, exists)
	_, exists = info.Inputs[inputSecret]
	assert.True(t, exists)
}

func TestSecretValueModule_Validate(t *testing.T) {
	module := NewSecretValueModule()
	fakeClientset := fake.NewClientset()
	namespace := "test-namespace"
	testSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: namespace,
		},
	}
	sec := &secret{
		s:  testSecret,
		si: fakeClientset.CoreV1().Secrets(namespace),
	}

	tests := []struct {
		name        string
		inputs      map[string]blackstart.Input
		expectError bool
	}{
		{
			name: "valid inputs",
			inputs: map[string]blackstart.Input{
				inputSecret: blackstart.NewInputFromValue(sec),
				inputKey:    blackstart.NewInputFromValue("test-key"),
				inputValue:  blackstart.NewInputFromValue("test-value"),
			},
			expectError: false,
		},
		{
			name: "missing secret",
			inputs: map[string]blackstart.Input{
				inputKey:   blackstart.NewInputFromValue("test-key"),
				inputValue: blackstart.NewInputFromValue("test-value"),
			},
			expectError: true,
		},
		{
			name: "missing key",
			inputs: map[string]blackstart.Input{
				inputSecret: blackstart.NewInputFromValue(sec),
				inputValue:  blackstart.NewInputFromValue("test-value"),
			},
			expectError: true,
		},
		{
			name: "missing value",
			inputs: map[string]blackstart.Input{
				inputSecret: blackstart.NewInputFromValue(sec),
				inputKey:    blackstart.NewInputFromValue("test-key"),
			},
			expectError: true,
		},
		{
			name: "empty string key",
			inputs: map[string]blackstart.Input{
				inputSecret: blackstart.NewInputFromValue(sec),
				inputKey:    blackstart.NewInputFromValue(""),
				inputValue:  blackstart.NewInputFromValue("test-value"),
			},
			expectError: true,
		},
		{
			name: "empty string value is valid",
			inputs: map[string]blackstart.Input{
				inputSecret: blackstart.NewInputFromValue(sec),
				inputKey:    blackstart.NewInputFromValue("test-key"),
				inputValue:  blackstart.NewInputFromValue(""),
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				operation := blackstart.Operation{
					Module: "kubernetes_secret_value",
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

func TestSecretValueModule_Check(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewClientset()

	// Create test Secret
	initialSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"existing-key": []byte("existing-value"),
		},
	}
	_, err := clientset.CoreV1().Secrets("test-namespace").Create(
		context.Background(),
		initialSecret,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	module := NewSecretValueModule()

	tests := []struct {
		name           string
		secretName     string
		namespace      string
		key            string
		value          string
		doesNotExist   bool
		tainted        bool
		expectedResult bool
	}{
		{
			name:           "existing secret missing key",
			secretName:     "test-secret",
			namespace:      "test-namespace",
			key:            "new-key",
			value:          "new-value",
			expectedResult: false,
		},
		{
			name:           "existing secret existing key incorrect value",
			secretName:     "test-secret",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "different-value",
			expectedResult: false,
		},
		{
			name:           "existing secret existing key correct value",
			secretName:     "test-secret",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "existing-value",
			expectedResult: true,
		},
		{
			name:           "missing secret",
			secretName:     "missing",
			namespace:      "test-namespace",
			key:            "some-key",
			value:          "some-value",
			expectedResult: false,
		},
		{
			name:           "does not exist missing key",
			secretName:     "test-secret",
			namespace:      "test-namespace",
			key:            "missing-key",
			value:          "any-value",
			doesNotExist:   true,
			expectedResult: true,
		},
		{
			name:           "does not exist mode, existing key",
			secretName:     "test-secret",
			namespace:      "test-namespace",
			key:            "existing-key",
			value:          "any-value",
			doesNotExist:   true,
			expectedResult: false,
		},
		{
			name:           "tainted existing secret correct value",
			secretName:     "test-secret",
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
				// Get the Secret and create a secret object
				var sec *corev1.Secret
				var tErr error
				if test.secretName == "test-secret" {
					sec, tErr = clientset.CoreV1().Secrets(test.namespace).Get(
						context.Background(),
						test.secretName,
						metav1.GetOptions{},
					)
					require.NoError(t, tErr)
				} else {
					sec = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      test.secretName,
							Namespace: test.namespace,
						},
					}
				}

				secretObj := &secret{
					s:  sec,
					si: clientset.CoreV1().Secrets(test.namespace),
				}

				// Create inputs
				inputs := map[string]blackstart.Input{
					inputSecret: blackstart.NewInputFromValue(secretObj),
					inputKey:    blackstart.NewInputFromValue(test.key),
					inputValue:  blackstart.NewInputFromValue(test.value),
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
				require.NoError(t, tErr)

				// Verify result
				assert.Equal(t, test.expectedResult, result)
			},
		)
	}
}

func TestSecretValueModule_Set(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewClientset()

	// Create test Secret
	initialSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"existing-key": []byte("existing-value"),
		},
	}
	_, err := clientset.CoreV1().Secrets("test-namespace").Create(
		context.Background(),
		initialSecret,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	module := NewSecretValueModule()

	tests := []struct {
		name         string
		secretName   string
		namespace    string
		key          string
		value        string
		doesNotExist bool
		checkAfter   func(t *testing.T, clientset *fake.Clientset)
	}{
		{
			name:       "update existing Secret with new key",
			secretName: "test-secret",
			namespace:  "test-namespace",
			key:        "new-key",
			value:      "new-value",
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				sec, tErr := clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"test-secret",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "new-value", string(sec.Data["new-key"]))
				assert.Equal(t, "existing-value", string(sec.Data["existing-key"]))
			},
		},
		{
			name:       "update existing Secret with existing key",
			secretName: "test-secret",
			namespace:  "test-namespace",
			key:        "existing-key",
			value:      "updated-value",
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				sec, tErr := clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"test-secret",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "updated-value", string(sec.Data["existing-key"]))
			},
		},
		{
			name:         "delete key in does not exist mode",
			secretName:   "test-secret",
			namespace:    "test-namespace",
			key:          "existing-key",
			value:        "any-value",
			doesNotExist: true,
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				sec, tErr := clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"test-secret",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				_, exists := sec.Data["existing-key"]
				assert.False(t, exists)
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				// Get the Secret if it already exists, or create a new one
				var sec *corev1.Secret
				var tEerr error

				if test.secretName == "test-secret" {
					sec, tEerr = clientset.CoreV1().Secrets(test.namespace).Get(
						context.Background(),
						test.secretName,
						metav1.GetOptions{},
					)
					require.NoError(t, tEerr)
				} else {
					// For new Secret, create a new object (but don't add to clientset yet)
					sec = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      test.secretName,
							Namespace: test.namespace,
						},
					}
				}

				secretObj := &secret{
					s:  sec,
					si: clientset.CoreV1().Secrets(test.namespace),
				}

				// Create inputs
				inputs := map[string]blackstart.Input{
					inputSecret: blackstart.NewInputFromValue(secretObj),
					inputKey:    blackstart.NewInputFromValue(test.key),
					inputValue:  blackstart.NewInputFromValue(test.value),
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

func TestSecretValueModule(t *testing.T) {
	clientset := fake.NewClientset()

	module := NewSecretValueModule()

	testNamespace := "test-namespace"
	testSecretName := "test-secret"
	testKey := "test-key"
	testValue := "test-value"

	t.Run(
		"secret lifecycle", func(t *testing.T) {
			// Check for positive and negative does not exist variations of a missing secret

			nonExistentK8sSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSecretName,
					Namespace: testNamespace,
				},
			}

			nonExistentSecret := &secret{
				s:  nonExistentK8sSecret,
				si: clientset.CoreV1().Secrets(testNamespace),
			}

			// Create inputs for testing missing Secret
			inputs := map[string]blackstart.Input{
				inputSecret: blackstart.NewInputFromValue(nonExistentSecret),
				inputKey:    blackstart.NewInputFromValue(testKey),
				inputValue:  blackstart.NewInputFromValue(testValue),
			}

			// Check for a missing Secret (regular mode)
			ctx := blackstart.InputsToContext(context.Background(), inputs)
			result, err := module.Check(ctx)
			require.NoError(t, err)
			assert.False(t, result)

			// Check for a missing Secret in "does not exist" mode
			ctx = blackstart.InputsToContext(context.Background(), inputs, blackstart.DoesNotExistFlag)
			result, err = module.Check(ctx)
			require.NoError(t, err)
			assert.True(t, result)
		},
	)
}
