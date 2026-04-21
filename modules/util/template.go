package util

import (
	"bytes"
	"fmt"
	"reflect"
	"text/template"

	"github.com/pezops/blackstart"
)

const (
	moduleIDTemplate = "util_template"
	inputTemplate    = "template"
	outputResult     = "result"
)

func init() {
	blackstart.RegisterModule(moduleIDTemplate, NewTemplate)
}

// NewTemplate creates a module that renders a template into a string.
func NewTemplate() blackstart.Module {
	return &templateModule{}
}

type templateModule struct{}

func (m *templateModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   moduleIDTemplate,
		Name: "Template",
		Description: "Renders a templated string. Supports `workflowOutput \"<operation-id>\" " +
			"\"<output-key>\"` for reading outputs from operations in the current workflow run.",
		Requirements: []string{
			"Each operation referenced by `workflowOutput` must be listed in the template operation `dependsOn`.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputTemplate: {
				Description: "Go template format string to render.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputResult: {
				Description: "Rendered template result.",
				Type:        reflect.TypeFor[string](),
			},
		},
		Examples: map[string]string{
			"Render SQL IAM username from dependency outputs": `operations:
  - id: identity
    module: google_cloud_metadata
    inputs:
      requests:
        - project_id

  - id: sql-iam-username
    module: util_template
    dependsOn:
      - identity
    inputs:
      template: 'blackstart-sa@{{ workflowOutput "identity" "project_id" }}.iam'`,
		},
	}
}

func (m *templateModule) Validate(op blackstart.Operation) error {
	input, ok := op.Inputs[inputTemplate]
	if !ok {
		return fmt.Errorf("missing required parameter: %s", inputTemplate)
	}
	if !input.IsStatic() {
		return nil
	}
	_, err := blackstart.InputAs[string](input, true)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputTemplate, err)
	}
	return nil
}

func (m *templateModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.DoesNotExist() {
		return false, fmt.Errorf("doesNotExist is not supported by %s", moduleIDTemplate)
	}
	return false, nil
}

func (m *templateModule) Set(ctx blackstart.ModuleContext) error {
	if ctx.DoesNotExist() {
		return fmt.Errorf("doesNotExist is not supported by %s", moduleIDTemplate)
	}

	tplInput, err := blackstart.ContextInputAs[string](ctx, inputTemplate, true)
	if err != nil {
		return err
	}

	tpl, err := template.New(moduleIDTemplate).
		Option("missingkey=error").
		Funcs(
			template.FuncMap{
				"workflowOutput": func(operationID, outputKey string) (any, error) {
					return blackstart.ContextWorkflowOutput(ctx, operationID, outputKey)
				},
			},
		).
		Parse(tplInput)
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	var out bytes.Buffer
	if err := tpl.Execute(&out, nil); err != nil {
		return fmt.Errorf("failed rendering template: %w", err)
	}

	if err := ctx.Output(outputResult, out.String()); err != nil {
		return err
	}
	return nil
}
