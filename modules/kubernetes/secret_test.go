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

func TestSecretModule_Info(t *testing.T) {
	module := NewSecretModule()
	info := module.Info()

	assert.Equal(t, "kubernetes_secret", info.Id)

	// Check required inputs
	_, exists := info.Inputs[inputName]
	assert.True(t, exists)
	_, exists = info.Inputs[inputNamespace]
	assert.True(t, exists)
	_, exists = info.Inputs[inputClient]
	assert.True(t, exists)
	_, exists = info.Inputs[inputType]
	assert.True(t, exists)
	_, exists = info.Inputs[inputImmutable]
	assert.True(t, exists)

	// Check outputs
	_, exists = info.Outputs[outputSecret]
	assert.True(t, exists)
}

func TestSecretModule_Validate(t *testing.T) {
	module := NewSecretModule()
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
				inputName:      blackstart.NewInputFromValue("test-secret"),
				inputNamespace: blackstart.NewInputFromValue("test-namespace"),
			},
			expectError: false,
		},
		{
			name: "missing client",
			inputs: map[string]blackstart.Input{
				inputName:      blackstart.NewInputFromValue("test-secret"),
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
				inputName:   blackstart.NewInputFromValue("test-secret"),
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				operation := blackstart.Operation{
					Module: "kubernetes_secret",
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

func TestSecretModule_Check(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewClientset()

	// Create test Secret
	initialSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}
	_, err := clientset.CoreV1().Secrets("test-namespace").Create(
		context.Background(),
		initialSecret,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Create test TLS Secret
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tls-secret",
			Namespace: "test-namespace",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
		},
	}
	_, err = clientset.CoreV1().Secrets("test-namespace").Create(
		context.Background(),
		tlsSecret,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	module := NewSecretModule()

	tests := []struct {
		name           string
		secretName     string
		namespace      string
		secretType     string
		doesNotExist   bool
		tainted        bool
		expectedResult bool
		expectError    bool
	}{
		{
			name:           "existing secret",
			secretName:     "test-secret",
			namespace:      "test-namespace",
			secretType:     string(corev1.SecretTypeOpaque),
			expectedResult: true,
		},
		{
			name:           "missing secret",
			secretName:     "missing",
			namespace:      "test-namespace",
			secretType:     string(corev1.SecretTypeOpaque),
			expectedResult: false,
			expectError:    true,
		},
		{
			name:           "does not exist missing secret",
			secretName:     "missing",
			namespace:      "test-namespace",
			secretType:     string(corev1.SecretTypeOpaque),
			doesNotExist:   true,
			expectedResult: true,
		},
		{
			name:           "does not exist existing secret",
			secretName:     "test-secret",
			namespace:      "test-namespace",
			secretType:     string(corev1.SecretTypeOpaque),
			doesNotExist:   true,
			expectedResult: false,
		},
		{
			name:           "tainted existing secret",
			secretName:     "test-secret",
			namespace:      "test-namespace",
			secretType:     string(corev1.SecretTypeOpaque),
			tainted:        true,
			expectedResult: false,
		},
		{
			name:           "tls secret type",
			secretName:     "test-tls-secret",
			namespace:      "test-namespace",
			secretType:     "kubernetes.io/tls",
			expectedResult: true,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				// Create inputs
				inputs := map[string]blackstart.Input{
					inputClient:    blackstart.NewInputFromValue(clientset),
					inputName:      blackstart.NewInputFromValue(test.secretName),
					inputNamespace: blackstart.NewInputFromValue(test.namespace),
					inputType:      blackstart.NewInputFromValue(test.secretType),
					inputImmutable: blackstart.NewInputFromValue((*bool)(nil)),
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

func TestSecretModule_Set(t *testing.T) {
	clientset := fake.NewClientset()

	initialSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}
	_, err := clientset.CoreV1().Secrets("test-namespace").Create(
		context.Background(),
		initialSecret,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	module := NewSecretModule()

	tests := []struct {
		name         string
		secretName   string
		namespace    string
		secretType   string
		doesNotExist bool
		checkAfter   func(t *testing.T, clientset *fake.Clientset)
	}{
		{
			name:       "create secret with default type",
			secretName: "new-secret",
			namespace:  "test-namespace",
			secretType: string(corev1.SecretTypeOpaque),
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var testSecret *corev1.Secret
				var tErr error
				testSecret, tErr = clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"new-secret",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "new-secret", testSecret.Name)
				assert.Equal(t, corev1.SecretTypeOpaque, testSecret.Type)
			},
		},
		{
			name:       "create secret with tls type",
			secretName: "tls-secret",
			namespace:  "test-namespace",
			secretType: string(corev1.SecretTypeTLS),
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var testSecret *corev1.Secret
				var tErr error
				testSecret, tErr = clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"tls-secret",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "tls-secret", testSecret.Name)
				assert.Equal(t, corev1.SecretTypeTLS, testSecret.Type)
			},
		},
		{
			name:       "create secret with dockerconfigjson type",
			secretName: "docker-secret",
			namespace:  "test-namespace",
			secretType: string(corev1.SecretTypeDockerConfigJson),
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var testSecret *corev1.Secret
				var tErr error
				testSecret, tErr = clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"docker-secret",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "docker-secret", testSecret.Name)
				assert.Equal(t, corev1.SecretTypeDockerConfigJson, testSecret.Type)
			},
		},
		{
			name:       "existing secret",
			secretName: "test-secret",
			namespace:  "test-namespace",
			secretType: string(corev1.SecretTypeOpaque),
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var testSecret *corev1.Secret
				var tErr error
				testSecret, tErr = clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"test-secret",
					metav1.GetOptions{},
				)
				require.NoError(t, tErr)
				assert.Equal(t, "test-secret", testSecret.Name)
				assert.Equal(t, corev1.SecretTypeOpaque, testSecret.Type)
			},
		},
		{
			name:         "does not exist delete secret",
			secretName:   "test-secret",
			namespace:    "test-namespace",
			secretType:   string(corev1.SecretTypeOpaque),
			doesNotExist: true,
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var tErr error
				_, tErr = clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"test-secret",
					metav1.GetOptions{},
				)
				require.True(t, apierrors.IsNotFound(tErr))
			},
		},
		{
			name:         "does not exist missing secret",
			secretName:   "missing-secret",
			namespace:    "test-namespace",
			secretType:   string(corev1.SecretTypeOpaque),
			doesNotExist: true,
			checkAfter: func(t *testing.T, clientset *fake.Clientset) {
				var tErr error
				_, tErr = clientset.CoreV1().Secrets("test-namespace").Get(
					context.Background(),
					"missing-secret",
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
					inputName:      blackstart.NewInputFromValue(test.secretName),
					inputNamespace: blackstart.NewInputFromValue(test.namespace),
					inputType:      blackstart.NewInputFromValue(test.secretType),
					inputImmutable: blackstart.NewInputFromValue((*bool)(nil)),
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

// TestSecretModule_Immutable uses envtest to spin up a real Kubernetes API server to test the
// immutable secret functionality with actual API validation. It performs a sequence of operations
// to create a secret, add values, make it immutable, and then attempts to modify it to ensure the
// immutability is enforced.
func TestSecretModule_Immutable(t *testing.T) {
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

	sm := NewSecretModule()

	testSecretName := "test-immutable-secret"
	namespace := "test-namespace"

	t.Run(
		"immutable_lifecycle", func(t *testing.T) {
			// Create a secret without specifying immutable (nil/default)
			inputs := map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(clientset),
				inputName:      blackstart.NewInputFromValue(testSecretName),
				inputNamespace: blackstart.NewInputFromValue(namespace),
				inputType:      blackstart.NewInputFromValue(string(corev1.SecretTypeOpaque)),
				inputImmutable: blackstart.NewInputFromValue((*bool)(nil)),
			}

			moduleCtx := blackstart.InputsToContext(ctx, inputs)
			err = sm.Set(moduleCtx)
			require.NoError(t, err)

			// Verify the secret was created
			var sec corev1.Secret
			err = k8sClient.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: testSecretName}, &sec)
			require.NoError(t, err)

			// Add test values to the secret using direct Kubernetes client
			// Get the current secret
			var testSecret *corev1.Secret
			testSecret, err = clientset.CoreV1().Secrets(namespace).Get(
				ctx,
				testSecretName,
				metav1.GetOptions{},
			)
			require.NoError(t, err)

			// Add some data
			if testSecret.Data == nil {
				testSecret.Data = make(map[string][]byte)
			}
			testSecret.Data["key1"] = []byte("value1")
			testSecret.Data["key2"] = []byte("value2")

			// Update the secret
			_, err = clientset.CoreV1().Secrets(namespace).Update(
				ctx,
				testSecret,
				metav1.UpdateOptions{},
			)
			require.NoError(t, err)

			// Verify both values were added
			err = k8sClient.Get(
				ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: testSecretName}, &sec,
			)
			require.NoError(t, err)

			assert.Equal(t, "value1", string(testSecret.Data["key1"]))
			assert.Equal(t, "value2", string(testSecret.Data["key2"]))

			// Use kubernetes_secret module to make it immutable
			trueVal := true
			inputs[inputImmutable] = blackstart.NewInputFromValue(&trueVal)
			moduleCtx = blackstart.InputsToContext(ctx, inputs)
			err = sm.Set(moduleCtx)
			require.NoError(t, err)

			// Verify the secret is now immutable
			err = k8sClient.Get(
				ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: testSecretName}, &sec,
			)
			require.NoError(t, err)
			// Immutable field should be *true
			assert.Equal(t, &trueVal, sec.Immutable)

			// Verify Check returns false when immutable field doesn't match
			// Check when using immutable: false, should return false
			falseVal := false
			checkInputs := map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(clientset),
				inputName:      blackstart.NewInputFromValue(testSecretName),
				inputNamespace: blackstart.NewInputFromValue(namespace),
				inputType:      blackstart.NewInputFromValue(string(corev1.SecretTypeOpaque)),
				inputImmutable: blackstart.NewInputFromValue(&falseVal),
			}
			checkCtx := blackstart.InputsToContext(ctx, checkInputs)

			var result bool
			result, err = sm.Check(checkCtx)
			require.NoError(t, err)
			assert.False(t, result)

			// Check with immutable: true, should return true
			checkInputs[inputImmutable] = blackstart.NewInputFromValue(&trueVal)
			checkCtx = blackstart.InputsToContext(ctx, checkInputs)
			result, err = sm.Check(checkCtx)
			require.NoError(t, err)
			assert.True(t, result)

			// Check with immutable: nil should return true (immutable field is ignored)
			checkInputsNil := map[string]blackstart.Input{
				inputClient:    blackstart.NewInputFromValue(clientset),
				inputName:      blackstart.NewInputFromValue(testSecretName),
				inputNamespace: blackstart.NewInputFromValue(namespace),
				inputType:      blackstart.NewInputFromValue(string(corev1.SecretTypeOpaque)),
				inputImmutable: blackstart.NewInputFromValue(nil),
			}
			checkCtx = blackstart.InputsToContext(ctx, checkInputsNil)
			result, err = sm.Check(checkCtx)
			require.NoError(t, err)
			assert.True(t, result)

			// Attempt to modify the secret - this should fail due to immutability
			// Get the current secret
			testSecret, err = clientset.CoreV1().Secrets(namespace).Get(
				ctx,
				testSecretName,
				metav1.GetOptions{},
			)
			require.NoError(t, err)

			// Try to modify an existing value
			testSecret.Data["key1"] = []byte("modified-value")

			// This update should fail because the secret is immutable
			_, err = clientset.CoreV1().Secrets(namespace).Update(
				ctx,
				testSecret,
				metav1.UpdateOptions{},
			)
			require.Error(t, err)
			assert.True(t, apierrors.IsInvalid(err))
		},
	)
}
