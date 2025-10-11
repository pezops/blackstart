package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/api/v1alpha1"
	_ "github.com/pezops/blackstart/internal/all_modules"
	"github.com/pezops/blackstart/util"
)

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
		err = runWorkflowsInK8s(ctx, kubeClient)
	}
	return
}

// runWorkflowFromFile loads a workflow from a file and runs it.
func runWorkflowFromFile(ctx context.Context) (err error) {
	logger := loggerFromCtx(ctx)

	var wf *blackstart.Workflow
	wf, err = loadWorkflowFromFile(ctx)
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
	config := configFromCtx(ctx)

	logger.Info("loading workflow resources from kubernetes")

	var workflows []*blackstart.Workflow
	namespaces := strings.Split(config.KubeNamespace, ",")
	if len(namespaces) == 0 {
		namespaces = []string{""}
	}
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
			err = nil
			return

		}
		workflows = append(workflows, nsWorkflows...)
	}

	var wfErrors []error
	var wg sync.WaitGroup
	for _, kwf := range workflows {
		wg.Add(1)
		go func(kwf *blackstart.Workflow) {
			defer wg.Done()
			wErr := runWorkflowInK8s(ctx, kubeClient, kwf)
			if wErr != nil {
				wfErrors = append(wfErrors, wErr)
			}
		}(kwf)
	}
	wg.Wait()
	if len(wfErrors) > 0 {
		err = fmt.Errorf("errors running workflows: %v", wfErrors)
	}

	return
}

// runWorkflowInK8s executes a single workflow and updates its Kubernetes status.
func runWorkflowInK8s(ctx context.Context, c client.Client, wf *blackstart.Workflow) error {
	logger := loggerFromCtx(ctx)
	start := time.Now()
	result := wf.Run(ctx)
	resultMsg := ""
	lastOpStart := ""

	if result.Err != nil {
		logger.Warn(
			"workflow execution did not complete", "workflow", wf.Name, "phase", result.Phase, "operation",
			result.Op.Id, "error", result.Err.Error(),
		)
		lastOpStart = result.Op.Id
		resultMsg = result.Err.Error()
	} else {
		logger.Info("workflow execution complete", "workflow", wf.Name)
	}

	// Update the workflow status in Kubernetes.
	status := v1alpha1.WorkflowStatus{
		LastRan:             metav1.NewTime(start),
		Successful:          strconv.FormatBool(result.Err == nil),
		Phase:               result.Phase,
		Result:              resultMsg,
		OperationsCompleted: fmt.Sprintf("%d/%d", result.CompletedOperations, result.TotalOperations),
		LastOperation:       lastOpStart,
	}
	err := updateWorkflowStatusInK8s(ctx, c, wf, status)
	if err != nil {
		logger.Error("error updating workflow status", "workflow", wf.Name, "error", err)
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
		t := reflect.TypeOf(v1alpha1.Workflow{}).String()
		return fmt.Errorf("unexpected workflow source: %v", t)
	}

	kwf.Status = status
	err := c.Status().Update(ctx, kwf)
	if err != nil {
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
	for i, kwf := range workflowList.Items {
		bsWf := new(blackstart.Workflow)
		bsWf.Name = kwf.Name
		bsWf.Description = kwf.Spec.Description
		bsWf.Operations, err = loadOperations(kwf.Spec.Operations)
		if err != nil {
			return nil, fmt.Errorf("error loading operations for workflow %s: %w", kwf.Name, err)
		}
		bsWf.Source = &kwf
		workflows[i] = bsWf
	}

	return workflows, nil
}

// loadWorkflowFromFile reads a workflow definition from a file and converts it to a core Workflow.
func loadWorkflowFromFile(ctx context.Context) (*blackstart.Workflow, error) {
	var err error

	logger := loggerFromCtx(ctx)
	config := configFromCtx(ctx)

	// Load workflow from file
	logger.Info("loading workflow file", "file", config.WorkflowFile)
	var workflowConfig []byte
	workflowConfig, err = os.ReadFile(config.WorkflowFile)
	if err != nil {
		err = fmt.Errorf("error reading workflow file: %w", err)
		return nil, err
	}

	var apiWf v1alpha1.WorkflowConfigFile
	err = yaml.Unmarshal(workflowConfig, &apiWf)
	if err != nil {
		err = fmt.Errorf("error unmarshalling workflow: %w", err)
		return nil, err
	}

	var wf blackstart.Workflow
	wf.Name = apiWf.Name
	wf.Description = apiWf.Description
	wf.Operations, err = loadOperations(apiWf.Operations)
	if err != nil {
		return nil, fmt.Errorf("error loading operations for workflow %s: %w", wf.Name, err)
	}
	wf.Source = apiWf
	return &wf, nil
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
				err = json.Unmarshal(v.Extra.Raw, &val)
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
