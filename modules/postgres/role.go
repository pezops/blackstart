package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"text/template"

	"github.com/pezops/blackstart"
)

func init() {
	blackstart.RegisterModule("postgres_role", NewPostgresRole)
}

func NewPostgresRole() blackstart.Module {
	return &roleModule{}
}

func newRole(name string) *role {
	return &role{
		Name:    name,
		Inherit: true,
	}
}

// role represents a role in a PostgreSQL database
type role struct {
	// Role or username of the role to operate on
	Name string
	// CreateDb is a boolean that determines if the role can create databases
	CreateDb bool
	// CreateRole is a boolean that determines if the role can create other roles
	CreateRole bool
	// Inherit is a boolean that determines if the role can inherit privileges from other roles
	Inherit bool
	// Login is a boolean that determines if the role can log in to the database
	Login bool
	// Replication is a boolean that determines if the role can initiate streaming replication
	Replication bool
}

type roleModule struct {
	op     *blackstart.Operation
	db     *sql.DB
	target *role
}

func (r roleModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "postgres_role",
		Name:        "PostgreSQL Role",
		Description: "Module to manage PostgreSQL Roles.",
		Requirements: []string{
			"A valid Postgres `connection` input must be provided.",
			"The executing database user must be a member of a role with `CREATEROLE`.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputName: {
				Description: "Id of the Role to manage.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputLogin: {
				Description: "If true, the Role can log in to the database.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
			},
			inputInherit: {
				Description: "If true, the Role can Inherit privileges from other roles.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
			},
			inputCreateDb: {
				Description: "If true, the Role can create databases.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
			},
			inputCreateRole: {
				Description: "If true, the Role can create other roles.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
			},
			inputReplication: {
				Description: "If true, the Role can initiate streaming Replication.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"Create a new Role": `id: create-role
module: postgres_role
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  Name: my-new-Role
  Login: true`,
		},
	}
}

func (r roleModule) Validate(op blackstart.Operation) error {

	for _, p := range requiredRoleParameters {
		if o, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		} else {
			if !o.IsStatic() {
				continue
			}
			_, err := blackstart.InputAs[string](o, true)
			if err != nil {
				return fmt.Errorf("parameter %s is invalid: %w", p, err)
			}
		}
	}

	return nil
}

func (r roleModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	err := r.createTargetRole(ctx)
	if err != nil {
		return false, err
	}

	roleExists, err := r.checkRoleExists(ctx)
	if err != nil {
		return false, err
	}

	if r.op.DoesNotExist {
		if roleExists {
			return false, nil
		}
		return true, nil
	}

	roleCorrect, err := r.checkRoleCorrectOptions(ctx)
	if err != nil {
		return false, err
	}
	return roleCorrect, nil
}

func (r roleModule) Set(ctx blackstart.ModuleContext) error {
	// We don't know if the Role already exists and is not setup correctly, or if it doesn't exist
	// at all. So we need to check both cases before setting the Role.
	roleExists, err := r.checkRoleExists(ctx)
	if err != nil {
		return err
	}

	if r.op.DoesNotExist && roleExists {
		return r.dropRole(ctx)
	}
	if !roleExists {
		return r.createRole(ctx)
	}
	return r.updateRole(ctx)
}

// createTargetRole creates the target Role from the operation inputs.
func (r roleModule) createTargetRole(ctx blackstart.ModuleContext) error {
	name, err := blackstart.ContextInputAs[string](ctx, inputName, true)
	if err != nil {
		return err
	}
	r.target = newRole(name)

	for _, p := range []string{inputLogin, inputInherit, inputCreateDb, inputCreateRole, inputReplication} {
		var v bool
		var inputVal blackstart.Input
		var useDefault bool
		inputVal, err = ctx.Input(p)
		if errors.Is(err, blackstart.ErrInputDoesNotExist) {
			useDefault = true
		} else if err != nil {
			return err
		} else {
			v, err = blackstart.InputAs[bool](inputVal, false)
			if err != nil {
				return fmt.Errorf("invalid input %s: %w", p, err)
			}
		}
		switch p {
		case inputLogin:
			if useDefault {
				r.target.Login = true
				continue
			}
			r.target.Login = v
		case inputInherit:
			if useDefault {
				r.target.Inherit = true
				continue
			}
			r.target.Inherit = v
		case inputCreateDb:
			if useDefault {
				r.target.CreateDb = false
				continue
			}
			r.target.CreateDb = v
		case inputCreateRole:
			if useDefault {
				r.target.CreateRole = false
				continue
			}
			r.target.CreateRole = v
		case inputReplication:
			if useDefault {
				r.target.Replication = false
				continue
			}
			r.target.Replication = v
		}
	}
	return nil
}

// checkRoleExists checks if the Role exists in the database.
func (r roleModule) checkRoleExists(ctx context.Context) (bool, error) {
	var err error
	queryParams := []interface{}{r.target.Name}

	// Execute the query
	var exists bool
	err = r.db.QueryRowContext(ctx, getRoleQuery, queryParams...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking grant: %w", err)
	}

	return exists, nil
}

// checkRoleCorrectOptions checks if the Role exists with the correct options.
func (r roleModule) checkRoleCorrectOptions(ctx context.Context) (bool, error) {
	var err error
	queryParams := []interface{}{
		r.target.Name, r.target.Inherit, r.target.CreateRole, r.target.CreateDb, r.target.Login, r.target.Replication,
	}

	// Execute the query to check if the correct Role exists
	var exists bool
	err = r.db.QueryRowContext(ctx, getRoleWithOptionsQuery, queryParams...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking Role: %w", err)
	}

	return exists, nil
}

// dropRole drops the Role from the database.
func (r roleModule) dropRole(ctx context.Context) error {
	tmpl, err := template.New("setRoleDelete").Parse(setRoleDeleteTemplate)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}
	var queryBuffer bytes.Buffer
	err = tmpl.Execute(&queryBuffer, r.target)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryBuffer.String())
	if err != nil {
		return fmt.Errorf("error dropping Role: %w", err)
	}

	return nil
}

// createRole creates the Role in the database.
func (r roleModule) createRole(ctx context.Context) error {
	tmpl, err := template.New("setRoleCreate").Parse(setRoleCreateTemplate)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}
	var queryBuffer bytes.Buffer
	err = tmpl.Execute(&queryBuffer, r.target)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryBuffer.String())
	if err != nil {
		return fmt.Errorf("error creating Role: %w", err)
	}

	return nil
}

// updateRole updates an existing role with the desired options.
func (r roleModule) updateRole(ctx context.Context) error {
	tmpl, err := template.New("setRoleUpdate").Parse(setRoleUpdateTemplate)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}
	var queryBuffer bytes.Buffer
	err = tmpl.Execute(&queryBuffer, r.target)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryBuffer.String())
	if err != nil {
		return fmt.Errorf("error updating Role: %w", err)
	}

	return nil
}
