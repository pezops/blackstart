package blackstart

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
)

var ErrOperationCycle = errors.New("operation cycle detected")

const (
	phaseSetup    = "Setup"
	phaseValidate = "Validate"
	phaseExecute  = "Execute"
)

// Workflow represents a series of operations to be executed. Each operation may depend on the
// outputs of other operations, forming a directed acyclic graph (DAG) of operations. The Workflow
// will be executed in an order that respects these dependencies.
// --8<-- [start:Workflow]
type Workflow struct {
	// Name is a simple Name or identifier for the Workflow.
	Name string `yaml:"name"`

	// Description is an optional field to describe the Workflow in greater detail.
	Description string `yaml:"description,omitempty"`

	// Operations is a map of operations that will be executed in the Workflow.
	Operations []Operation `yaml:"operations"`

	// Source is the original source of the workflow definition, if available.
	Source any
}

// --8<-- [end:Workflow]

// WorkflowResult represents the result of executing an operation. It contains the operation that was executed and any
// error that occurred during execution.
type WorkflowResult struct {
	Phase               string
	Op                  *Operation
	Err                 error
	TotalOperations     int
	CompletedOperations int
}

// Run will execute the Workflow using the provided context.
func (w *Workflow) Run(ctx context.Context) WorkflowResult {
	l := ctx.Value(LoggerKey)
	logger, ok := l.(*slog.Logger)
	if !ok {
		logger = NewLogger(nil)
	}
	we := newWorkflowExecution(w, logger)
	we.logger.Info("starting workflow execution")
	return we.execute(ctx)
}

// workflowExecution manages the execution of a Workflow. It keeps track of the operations,
// their contexts, and the overall state of the execution.
type workflowExecution struct {
	w      *Workflow
	opCtxs map[string]*moduleContext
	logger *slog.Logger
}

// execute runs the workflow by setting up operations, validating them, and executing them
// in the correct order based on their dependencies.
func (we *workflowExecution) execute(ctx context.Context) WorkflowResult {
	var err error
	var result WorkflowResult

	result.Phase = phaseSetup
	// Setup all operations and make sure all dependencies are captured.
	result.TotalOperations = len(we.w.Operations)
	for _, op := range we.w.Operations {
		err = op.setup()
		if err != nil {
			result.Err = err
			result.Op = &op
			return result
		}
	}

	// Create all modules and validate the operations.
	modules := make(map[string]Module)
	operations := make(map[string]*Operation)
	moduleInfo := make(map[string]ModuleInfo)

	// Instantiate modules for each operation and map them by operation Id.
	for _, op := range we.w.Operations {
		var m Module
		m, err = NewModule(&op)
		if err != nil {
			result.Err = fmt.Errorf("unable to instantiate module for operation: %w", err)
			result.Op = &op
			return result
		}

		modules[op.Id] = m
		operations[op.Id] = &op
		moduleInfo[op.Id] = m.Info()

	}

	// Topologically sort operations based on their dependencies.
	sortedIds, err := opoSort(we.w.Operations)
	if err != nil {
		result.Op = nil
		result.Err = fmt.Errorf("unable to sort operations: %w", err)
		return result
	}

	result.Phase = phaseValidate
	// Run input / output checks for each operation in their sorted order.
	for _, opId := range sortedIds {
		info, ok := moduleInfo[opId]
		if !ok {
			result.Err = fmt.Errorf("unable to find module info for operation '%s'", opId)
			return result
		}
		op := operations[opId]
		err = checkInputsOutputs(op, info, moduleInfo)
		if err != nil {
			result.Err = err
			result.Op = op
			return result
		}
	}

	// Validate each operation using its module.
	for _, op := range operations {
		result.Op = op
		m, ok := modules[op.Id]
		if !ok {
			result.Err = fmt.Errorf("unable to find module for operation '%s'", op.Id)
			return result
		}
		err = m.Validate(*op)
		if err != nil {
			result.Err = fmt.Errorf("validation failed for operation: %v: %w", op.Id, err)
			return result
		}
	}

	result.Phase = phaseExecute
	// Execute each operation in sorted order.
	operationContexts := make(map[string]ModuleContext)
	for _, id := range sortedIds {
		op := operations[id]
		result.Op = op
		mctx := newModuleContext(ctx, op)
		we.opCtxs[id] = mctx

		err = we.setupOperationContext(mctx, op)
		if err != nil {
			result.Err = fmt.Errorf("error setting up context: %w", err)
			return result
		}

		operationContexts[id] = mctx
		err = op.execute(mctx, we.logger)
		if err != nil {
			result.Err = err
			return result
		}
		result.CompletedOperations += 1
	}

	return result
}

// checkInputsOutputs will verify that all required inputs for an operation are present and
// that their types match the expected types defined in the module info. It will also check that
// any inputs that come from dependencies have matching output types from those dependencies. This
// helps find type mismatches and missing inputs before execution.
func checkInputsOutputs(op *Operation, info ModuleInfo, opsInfo map[string]ModuleInfo) error {
	// Check that all required inputs are present.
	for name, param := range info.Inputs {
		input, ok := op.Inputs[name]
		if !ok {
			if param.Required {
				return fmt.Errorf("missing required input %q for operation %q", name, op.Id)
			}
			continue
		}

		if input.IsStatic() {
			value := input.Any()
			if !matchesType(value, param.Type) {
				return fmt.Errorf(
					"input %q for operation %q is static but is not assignable to expected type %v",
					name, op.Id, param.Type,
				)
			}
		} else {
			var depInfo ModuleInfo
			var output OutputValue
			depId := input.DependencyId()
			// Get the output info from the dependency operation.
			depInfo, ok = opsInfo[depId]
			if !ok {
				return fmt.Errorf(
					"dependency operation %q for input %q in operation %q not found",
					depId, name, op.Id,
				)
			}
			outputKey := input.OutputKey()
			output, ok = depInfo.Outputs[outputKey]
			if !ok {
				return fmt.Errorf(
					"output %q from dependency operation %q for input %q in operation %q not found",
					outputKey, depId, name, op.Id,
				)
			}
			if output.Type != param.Type {
				return fmt.Errorf(
					"input %q for operation %q is does not match expected type %v from dependency %q",
					name, op.Id, param.Type, depId,
				)
			}
		}
	}
	return nil
}

// setupOperationContext will create a module context for each operation in the Workflow. It
// processes each input defined for the operation, and then sets the input values in the context
// for the operation. Inputs that come from dependencies are retrieved from the outputs of the
// previous operations.
func (we *workflowExecution) setupOperationContext(mctx *moduleContext, op *Operation) error {
	// Using the outputs from the dependency contexts (we.opCtxs), set the inputs for the current
	// operation. Also, set the inputs for values not from dependencies. All inputs should be set
	// using the setInput method.
	for k, input := range op.Inputs {
		if !input.IsStatic() {
			depOpCtx, ok := we.opCtxs[input.DependencyId()]
			if !ok {
				return fmt.Errorf(
					"dependency operation context not found: %v", input.DependencyId(),
				)
			}
			depOutput, err := depOpCtx.getOutput(input.OutputKey())
			if err != nil {
				return err
			}
			mctx.setInput(k, depOutput)
		}
	}

	return nil
}

// dependencyGraph represents a directed graph of operations and their dependencies.
type dependencyGraph struct {
	deps map[string][]string
	ops  []string
}

// addDep adds a dependency from one operation to another in the graph.
func (g *dependencyGraph) addDep(from, to string) {
	if g.deps == nil {
		g.deps = make(map[string][]string)
	}
	g.deps[from] = append(g.deps[from], to)
}

// dfs performs a depth-first search to detect cycles and build the topological order.
func (g *dependencyGraph) dfs(opId string, visited, recursionStack map[string]bool, currentOrder *[]string) error {
	visited[opId] = true
	recursionStack[opId] = true

	for _, depId := range g.deps[opId] {
		if recursionStack[depId] {
			// Cycle detected
			return fmt.Errorf("cycle involving %q: %w", depId, ErrOperationCycle)
		}
		if !visited[depId] {
			if err := g.dfs(depId, visited, recursionStack, currentOrder); err != nil {
				return err
			}
		}
	}

	recursionStack[opId] = false // Remove from recursion stack
	*currentOrder = append(*currentOrder, opId)
	return nil
}

// topoSort performs a topological sort of the operations in the graph. It returns an ordered
// list of operation IDs or an error if a cycle is detected.
func (g *dependencyGraph) topoSort() ([]string, error) {
	var order []string
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	for _, opId := range g.ops {
		if !visited[opId] {
			if err := g.dfs(opId, visited, recursionStack, &order); err != nil {
				return nil, err
			}
		}
	}

	return order, nil
}

// opoSort will topologically sort a set of operations by their id into a linear execution plan.
func opoSort(ops []Operation) ([]string, error) {
	g := &dependencyGraph{
		ops: make([]string, len(ops)),
	}
	for i, op := range ops {
		g.ops[i] = op.Id
		for _, dep := range op.DependsOn {
			g.addDep(op.Id, dep)
		}
	}

	return g.topoSort()
}

// newWorkflowExecution creates a new workflowExecution instance for the given Workflow.
func newWorkflowExecution(workflow *Workflow, logger *slog.Logger) *workflowExecution {
	logger = logger.With("workflow", workflow.Name)
	return &workflowExecution{
		w:      workflow,
		opCtxs: make(map[string]*moduleContext, len(workflow.Operations)),
		logger: logger,
	}
}

// matchesType checks if the value v matches the expected type t.
func matchesType(v any, t reflect.Type) bool {
	if v == nil {
		return false // untyped nil
	}
	vt := reflect.TypeOf(v)

	if t.Kind() == reflect.Interface {
		return vt.Implements(t)
	}

	// Check direct assignment
	if vt == t || vt.AssignableTo(t) {
		return true
	}

	// If target is a pointer type, check if the value type matches the pointer's element type
	// This allows bool to be assignable to *bool, string to *string, etc.
	if t.Kind() == reflect.Ptr {
		elemType := t.Elem()
		if vt == elemType || vt.AssignableTo(elemType) {
			return true
		}
	}

	return false
}
