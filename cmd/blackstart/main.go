package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/api/v1alpha1"
	_ "github.com/pezops/blackstart/internal/all_modules"
	"github.com/pezops/blackstart/util"
)

var Version = "dev"

const defaultReconcileInterval = 5 * time.Minute

func main() {
	config, err := blackstart.ReadConfig()
	if err != nil {
		var e *flags.Error
		if errors.As(err, &e) {
			if errors.Is(e.Type, flags.ErrHelp) {
				// If the error is just a help request, we can exit gracefully
				return
			}
		}
		_, _ = fmt.Fprintf(os.Stderr, "error reading configuration: %v", err.Error())
		os.Exit(1)
	}
	if config == nil {
		_, _ = fmt.Fprintf(os.Stdout, "configuration was empty")
		os.Exit(1)
	}

	if config.Version {
		fmt.Printf("Blackstart version: %s\n", Version)
		return
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	logger := blackstart.NewLogger(config)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, logger)
	ctx = context.WithValue(ctx, blackstart.ConfigKey, config)

	// Set up a channel to listen for OS signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Start a goroutine to handle the signals
	go func() {
		<-sigs
		cancel()
	}()

	ctx = loadK8sApiSchemes(ctx, logger)

	var kubeClient client.Client
	if config.WorkflowFile == "" {
		// Try to create a Kubernetes client to verify we can connect to the cluster
		kubeClient, err = workflowKubeClient(ctx)
		if err != nil {
			logger.Error("unable to create Kubernetes client", "error", err)
			os.Exit(1)
		}
	}

	err = run(ctx, kubeClient)
	if err != nil {
		logger.Error("error running blackstart", "error", err)
		os.Exit(1)
	}
}

// run executes the main logic of Blackstart, either running a single workflow from file or loading
// and running workflows from Kubernetes.
func run(ctx context.Context, kubeClient client.Client) (err error) {
	config := configFromCtx(ctx)

	if config.WorkflowFile != "" {
		err = runWorkflowFromFile(ctx)
	} else {
		var mode string
		mode, err = parseRuntimeMode(config.RuntimeMode)
		if err != nil {
			return err
		}
		if mode == "once" {
			err = runWorkflowsInK8s(ctx, kubeClient)
		} else {
			err = runWorkflowsControllerInK8s(ctx, kubeClient)
		}
	}
	return
}

func parseRuntimeMode(raw string) (string, error) {
	mode := strings.TrimSpace(raw)
	if mode == "" {
		return "controller", nil
	}
	if strings.EqualFold(mode, "controller") {
		return "controller", nil
	}
	if strings.EqualFold(mode, "once") {
		return "once", nil
	}
	return "", fmt.Errorf("invalid runtime mode %q: expected \"controller\" or \"once\"", raw)
}

// runWorkflowFromFile loads a workflow from a file and runs it.
func runWorkflowFromFile(ctx context.Context) (err error) {
	logger := loggerFromCtx(ctx)

	var wf *blackstart.Workflow
	wf, err = loadWorkflowFromSource(ctx)
	if err != nil {
		err = fmt.Errorf("error loading workflow from file: %w", err)
		return err
	}

	// Run the workflow
	res := wf.Run(ctx)
	if res.Err != nil {
		logger.Warn("workflow execution did not complete", "workflow", wf.Name, "error", res.Err.Error())
	} else {
		logger.Info("workflow execution complete", "workflow", wf.Name)
	}
	return
}

// runWorkflowsInK8s loads workflows from Kubernetes and runs them concurrently.
func runWorkflowsInK8s(ctx context.Context, kubeClient client.Client) (err error) {
	logger := loggerFromCtx(ctx)

	logger.Info("loading workflow resources from kubernetes")

	var workflows []*blackstart.Workflow
	namespaces := parseNamespaces(configFromCtx(ctx))
	for _, ns := range namespaces {
		var nsWorkflows []*blackstart.Workflow
		nsWorkflows, err = loadWorkflowsFromK8s(ctx, kubeClient, ns)
		if err != nil {
			err = fmt.Errorf("error loading workflows from Kubernetes: %w", err)
			return
		}

		if len(nsWorkflows) == 0 {
			if ns != "" {
				logger.Warn("no workflows found in namespace", "namespace", ns)
			} else {
				logger.Warn("no workflows found")
			}
			continue
		}
		workflows = append(workflows, nsWorkflows...)
	}
	if len(workflows) == 0 {
		logger.Warn("no workflows found in configured namespaces")
		return nil
	}

	errCh := make(chan error, len(workflows))
	var wg sync.WaitGroup
	for _, kwf := range workflows {
		wg.Add(1)
		go func(kwf *blackstart.Workflow) {
			defer wg.Done()
			wErr := runWorkflowInK8s(ctx, kubeClient, kwf)
			if wErr != nil {
				errCh <- wErr
			}
		}(kwf)
	}
	wg.Wait()
	close(errCh)

	var wfErrors []error
	for wErr := range errCh {
		wfErrors = append(wfErrors, wErr)
	}
	if len(wfErrors) > 0 {
		err = fmt.Errorf("errors running workflows: %v", wfErrors)
	}

	return
}

// runWorkflowInK8s executes a single workflow and updates its Kubernetes status.
func runWorkflowInK8s(ctx context.Context, c client.Client, wf *blackstart.Workflow) error {
	logger := loggerFromCtx(ctx)
	result := wf.Run(ctx)
	end := time.Now()
	resultMsg := ""
	lastError := ""
	lastOpStart := ""
	lastOpFields := []any{}
	if result.Op != nil {
		lastOpStart = result.Op.Id
		lastOpFields = append(lastOpFields, "operation", result.Op.Id)
	}

	if result.Err != nil {
		logFields := []any{
			"workflow", wf.Name,
			"namespace", wf.Namespace,
			"phase", result.Phase,
			"error", result.Err.Error(),
		}
		logFields = append(logFields, lastOpFields...)
		logger.Warn("workflow execution did not complete", logFields...)
		resultMsg = result.Err.Error()
		lastError = result.Err.Error()
	} else {
		logger.Info("workflow execution complete", "workflow", wf.Name, "namespace", wf.Namespace)
	}

	// Update the workflow status in Kubernetes.
	status := v1alpha1.WorkflowStatus{
		LastRan:             metav1.NewTime(end),
		NextRun:             metav1.NewTime(end.Add(wf.ReconcileInterval)),
		Successful:          strconv.FormatBool(result.Err == nil),
		Phase:               result.Phase,
		Result:              resultMsg,
		LastError:           lastError,
		OperationsCompleted: fmt.Sprintf("%d/%d", result.CompletedOperations, result.TotalOperations),
		LastOperation:       lastOpStart,
	}
	err := updateWorkflowStatusFunc(ctx, c, wf, status)
	if err != nil {
		logger.Error("error updating workflow status", "workflow", wf.Name, "namespace", wf.Namespace, "error", err)
	}
	return err
}

// updateWorkflowStatusInK8s updates the Workflow resource status in Kubernetes with the result of
// the Workflow run.
func updateWorkflowStatusInK8s(
	ctx context.Context, c client.Client, wf *blackstart.Workflow, status v1alpha1.WorkflowStatus,
) error {
	if wf.Source == nil {
		return fmt.Errorf("no workflow source")
	}
	kwf, ok := wf.Source.(*v1alpha1.Workflow)
	if !ok {
		return fmt.Errorf("unexpected workflow source type: %T", wf.Source)
	}

	key := types.NamespacedName{Name: kwf.Name, Namespace: kwf.Namespace}
	err := retry.RetryOnConflict(
		retry.DefaultBackoff, func() error {
			var latest v1alpha1.Workflow
			getErr := c.Get(ctx, key, &latest)
			if getErr != nil {
				return getErr
			}
			latest.Status = status
			updateErr := c.Status().Update(ctx, &latest)
			if updateErr != nil {
				return updateErr
			}
			kwf.Status = status
			return nil
		},
	)
	if err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("error updating workflow status after retries (conflict): %w", err)
		}
		return fmt.Errorf("error updating workflow status: %w", err)
	}
	return nil
}

// loggerFromCtx retrieves the logger from the context, or creates a new one if not found.
func loggerFromCtx(ctx context.Context) *slog.Logger {
	logger := ctx.Value(blackstart.LoggerKey).(*slog.Logger)
	if logger == nil {
		logger = blackstart.NewLogger(nil)
	}
	return logger
}

// configFromCtx retrieves the runtime configuration from the context.
func configFromCtx(ctx context.Context) *blackstart.RuntimeConfig {
	return ctx.Value(blackstart.ConfigKey).(*blackstart.RuntimeConfig)
}

// loadWorkflowsFromK8s is the entrypoint that retrieves Kubernetes CRD Workflow resources from
// the specified namespace and converts them to the native blackstart.Workflow instances.
//
// Currently, this a few other functions here only translate the `v1alpha1` API version. Future
// versions may be added here, and if any new revision of the native type requires an incompatible
// change, previous API versions will lose support. However, changes that are backward compatible
// will be able to support multiple API versions. For support purposes, this will require a
// transition period before any API version is removed from support.
func loadWorkflowsFromK8s(ctx context.Context, c client.Client, namespace string) ([]*blackstart.Workflow, error) {
	var workflowList v1alpha1.WorkflowList
	err := c.List(ctx, &workflowList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("error listing workflows: %w", err)
	}

	workflows := make([]*blackstart.Workflow, len(workflowList.Items))
	for i := range workflowList.Items {
		kwf := workflowList.Items[i].DeepCopy()
		bsWf, convErr := workflowFromK8sResource(kwf)
		if convErr != nil {
			return nil, convErr
		}
		workflows[i] = bsWf
	}

	return workflows, nil
}

func workflowFromK8sResource(kwf *v1alpha1.Workflow) (*blackstart.Workflow, error) {
	if kwf == nil {
		return nil, fmt.Errorf("workflow resource is nil")
	}
	wfRef := types.NamespacedName{Namespace: kwf.Namespace, Name: kwf.Name}.String()
	reconcileInterval, err := parseReconcileInterval(kwf.Spec.ReconcileInterval)
	if err != nil {
		return nil, fmt.Errorf("error parsing reconcile interval for workflow %s: %w", wfRef, err)
	}
	ops, err := loadOperations(kwf.Spec.Operations)
	if err != nil {
		return nil, fmt.Errorf("error loading operations for workflow %s: %w", wfRef, err)
	}

	return &blackstart.Workflow{
		Name:              kwf.Name,
		Namespace:         kwf.Namespace,
		Description:       kwf.Spec.Description,
		ReconcileInterval: reconcileInterval,
		Operations:        ops,
		Source:            kwf,
	}, nil
}

// loadWorkflowFromFile reads a workflow definition from a file and converts it to a core Workflow.
func loadWorkflowFromFile(ctx context.Context) (*blackstart.Workflow, error) {
	logger := loggerFromCtx(ctx)
	config := configFromCtx(ctx)

	// Load workflow from a local file path only.
	logger.Info("loading workflow file", "file", config.WorkflowFile)
	workflowConfig, err := os.ReadFile(config.WorkflowFile)
	if err != nil {
		return nil, fmt.Errorf("error reading workflow file: %w", err)
	}
	return workflowFromConfigBytes(workflowConfig)
}

// loadWorkflowFromEnv reads workflow YAML from an env var source
// BLACKSTART_WORKFLOW_FILE=env:<ENV_VAR_NAME>.
func loadWorkflowFromEnv(ctx context.Context) (*blackstart.Workflow, error) {
	logger := loggerFromCtx(ctx)
	config := configFromCtx(ctx)
	logger.Info("loading workflow from environment", "source", config.WorkflowFile)

	name := strings.TrimSpace(strings.TrimPrefix(config.WorkflowFile, "env:"))
	workflowConfig, err := workflowConfigBytesFromEnv(name)
	if err != nil {
		return nil, fmt.Errorf("error loading workflow from environment source: %w", err)
	}
	return workflowFromConfigBytes(workflowConfig)
}

// loadWorkflowFromGCS reads workflow YAML from a GCS source
// BLACKSTART_WORKFLOW_FILE=gs://<bucket>/<object>.
func loadWorkflowFromGCS(ctx context.Context) (*blackstart.Workflow, error) {
	logger := loggerFromCtx(ctx)
	config := configFromCtx(ctx)
	logger.Info("loading workflow from GCS", "source", config.WorkflowFile)

	bucket, object, err := parseGCSWorkflowSource(config.WorkflowFile)
	if err != nil {
		return nil, fmt.Errorf("error parsing GCS workflow source: %w", err)
	}
	workflowConfig, err := workflowConfigBytesFromGCS(ctx, bucket, object)
	if err != nil {
		return nil, fmt.Errorf("error loading workflow from GCS source: %w", err)
	}
	return workflowFromConfigBytes(workflowConfig)
}

func parseReconcileInterval(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultReconcileInterval, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid reconcileInterval %q: %w", raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid reconcileInterval %q: must be greater than 0", raw)
	}
	return d, nil
}

// loadOperations converts operations from configuration to core operations.
func loadOperations(ops []v1alpha1.Operation) ([]blackstart.Operation, error) {
	var err error
	bOps := make([]blackstart.Operation, len(ops))
	for i, op := range ops {
		coreOp := new(blackstart.Operation)
		coreOp.Name = op.Name
		coreOp.Module = op.Module
		coreOp.Id = op.Id
		coreOp.Description = op.Description
		coreOp.DependsOn = op.DependsOn
		coreOp.DoesNotExist = op.DoesNotExist
		coreOp.Tainted = op.Tainted
		coreOp.Inputs = make(map[string]blackstart.Input)
		for k, v := range op.Inputs {
			if v.Extra != nil && v.FromDependency == nil {
				var val any
				val, err = decodeOperationInputExtra(v.Extra.Raw)
				if err != nil {
					return nil, fmt.Errorf(
						"error unmarshalling input extra field for operation %s input %s: %w", op.Id, k, err,
					)
				}
				coreOp.Inputs[k] = blackstart.NewInputFromValue(val)
				continue
			}
			coreOp.Inputs[k] = blackstart.NewInputFromDep(v.FromDependency.Id, v.FromDependency.Output)
		}
		bOps[i] = *coreOp
	}
	return bOps, nil

}

// decodeOperationInputExtra decodes static operation input data captured in
// OperationInput.Extra.Raw. The raw bytes can come from YAML or JSON
// unmarshalling paths, so this attempts JSON first and then falls back to YAML.
func decodeOperationInputExtra(raw []byte) (any, error) {
	var val any
	if err := json.Unmarshal(raw, &val); err == nil {
		return val, nil
	}
	if err := yaml.Unmarshal(raw, &val); err == nil {
		return val, nil
	}
	return nil, fmt.Errorf("unable to decode raw input value")
}

// loadK8sApiSchemes initializes the Kubernetes API schemes and stores them in the context.
func loadK8sApiSchemes(ctx context.Context, logger *slog.Logger) context.Context {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	if err != nil {
		logger.Error("error adding v1alpha1 to scheme", "error", err)
		os.Exit(1)
	}
	return context.WithValue(ctx, blackstart.SchemeKey, scheme)
}

func workflowKubeClient(ctx context.Context) (client.Client, error) {
	var c client.Client

	scheme := ctx.Value(blackstart.SchemeKey).(*runtime.Scheme)

	clientConfig, err := util.GetK8sClientConfig()
	if err != nil {
		return nil, err
	}

	c, err = client.New(clientConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes client: %w", err)
	}
	return c, nil
}
