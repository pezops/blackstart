package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/pezops/blackstart"
	"github.com/stretchr/testify/require"
)

func TestDefaultPrivilegesBehavior_TableGrantAndRevoke(t *testing.T) {
	ctx := context.Background()
	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	owner := "blackstart_defpriv_owner_1"
	grantee := "blackstart_defpriv_grantee_1"

	setupSQL := []string{
		fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, grantee),
		fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, owner),
		fmt.Sprintf(`CREATE ROLE "%s";`, owner),
		fmt.Sprintf(`CREATE ROLE "%s";`, grantee),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public REVOKE SELECT ON TABLES FROM "%s" RESTRICT;`, owner, grantee),
	}
	for _, stmt := range setupSQL {
		_, err := db.ExecContext(ctx, stmt)
		require.NoError(t, err)
	}

	exists, err := defaultPrivilegeExists(
		ctx,
		db,
		owner,
		defaultACLObjTypeTables,
		"public",
		grantee,
		"SELECT",
		false,
	)
	require.NoError(t, err)
	require.False(t, exists)

	_, err = db.ExecContext(
		ctx,
		fmt.Sprintf(
			`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public GRANT SELECT ON TABLES TO "%s";`,
			owner,
			grantee,
		),
	)
	require.NoError(t, err)

	exists, err = defaultPrivilegeExists(
		ctx,
		db,
		owner,
		defaultACLObjTypeTables,
		"public",
		grantee,
		"SELECT",
		false,
	)
	require.NoError(t, err)
	require.True(t, exists)

	_, err = db.ExecContext(
		ctx,
		fmt.Sprintf(
			`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public REVOKE SELECT ON TABLES FROM "%s" RESTRICT;`,
			owner,
			grantee,
		),
	)
	require.NoError(t, err)

	exists, err = defaultPrivilegeExists(
		ctx,
		db,
		owner,
		defaultACLObjTypeTables,
		"public",
		grantee,
		"SELECT",
		false,
	)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestDefaultPrivilegesBehavior_TableGrantOptionAndRevoke(t *testing.T) {
	ctx := context.Background()
	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	owner := "blackstart_defpriv_owner_2"
	grantee := "blackstart_defpriv_grantee_2"

	setupSQL := []string{
		fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, grantee),
		fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, owner),
		fmt.Sprintf(`CREATE ROLE "%s";`, owner),
		fmt.Sprintf(`CREATE ROLE "%s";`, grantee),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public REVOKE SELECT ON TABLES FROM "%s" RESTRICT;`, owner, grantee),
	}
	for _, stmt := range setupSQL {
		_, err := db.ExecContext(ctx, stmt)
		require.NoError(t, err)
	}

	_, err := db.ExecContext(
		ctx,
		fmt.Sprintf(
			`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public GRANT SELECT ON TABLES TO "%s" WITH GRANT OPTION;`,
			owner,
			grantee,
		),
	)
	require.NoError(t, err)

	exists, err := defaultPrivilegeExists(
		ctx,
		db,
		owner,
		defaultACLObjTypeTables,
		"public",
		grantee,
		"SELECT",
		true,
	)
	require.NoError(t, err)
	require.True(t, exists)

	_, err = db.ExecContext(
		ctx,
		fmt.Sprintf(
			`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public REVOKE SELECT ON TABLES FROM "%s" CASCADE;`,
			owner,
			grantee,
		),
	)
	require.NoError(t, err)

	exists, err = defaultPrivilegeExists(
		ctx,
		db,
		owner,
		defaultACLObjTypeTables,
		"public",
		grantee,
		"SELECT",
		false,
	)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestDefaultPrivilegesBehavior_PreRevokeCascadeThenGrant(t *testing.T) {
	ctx := context.Background()
	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	owner := "blackstart_defpriv_owner_3"
	grantee := "blackstart_defpriv_grantee_3"

	setupSQL := []string{
		fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, grantee),
		fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, owner),
		fmt.Sprintf(`CREATE ROLE "%s";`, owner),
		fmt.Sprintf(`CREATE ROLE "%s";`, grantee),
	}
	for _, stmt := range setupSQL {
		_, err := db.ExecContext(ctx, stmt)
		require.NoError(t, err)
	}

	// Preemptive revoke before any default grant exists.
	_, err := db.ExecContext(
		ctx,
		fmt.Sprintf(
			`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public REVOKE SELECT ON TABLES FROM "%s" CASCADE;`,
			owner,
			grantee,
		),
	)
	require.NoError(t, err)

	exists, err := defaultPrivilegeExists(
		ctx,
		db,
		owner,
		defaultACLObjTypeTables,
		"public",
		grantee,
		"SELECT",
		false,
	)
	require.NoError(t, err)
	require.False(t, exists)

	_, err = db.ExecContext(
		ctx,
		fmt.Sprintf(
			`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public GRANT SELECT ON TABLES TO "%s";`,
			owner,
			grantee,
		),
	)
	require.NoError(t, err)

	exists, err = defaultPrivilegeExists(
		ctx,
		db,
		owner,
		defaultACLObjTypeTables,
		"public",
		grantee,
		"SELECT",
		false,
	)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestDefaultPrivilegesModule_TableGrantAndRevokeModes(t *testing.T) {
	ctx := context.Background()
	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	owner := "blackstart_defpriv_module_owner_1"
	grantee := "blackstart_defpriv_module_grantee_1"

	setupSQL := []string{
		fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, grantee),
		fmt.Sprintf(`DROP ROLE IF EXISTS "%s";`, owner),
		fmt.Sprintf(`CREATE ROLE "%s";`, owner),
		fmt.Sprintf(`CREATE ROLE "%s";`, grantee),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA public REVOKE SELECT ON TABLES FROM "%s" RESTRICT;`, owner, grantee),
	}
	for _, stmt := range setupSQL {
		_, err := db.ExecContext(ctx, stmt)
		require.NoError(t, err)
	}

	m := newDefaultPrivilegesModule()

	grantInputs := map[string]blackstart.Input{
		inputConnection:      blackstart.NewInputFromValue(db),
		inputRole:            blackstart.NewInputFromValue(grantee),
		inputPermission:      blackstart.NewInputFromValue("SELECT"),
		inputScope:           blackstart.NewInputFromValue(string(defaultPrivilegeScopeTables)),
		inputForRole:         blackstart.NewInputFromValue(owner),
		inputSchema:          blackstart.NewInputFromValue("public"),
		inputWithGrantOption: blackstart.NewInputFromValue(true),
	}
	grantOp := blackstart.Operation{
		Id:     "default-priv-grant",
		Module: defaultPrivilegesModuleID,
		Inputs: grantInputs,
	}
	require.NoError(t, m.Validate(grantOp))

	grantCtx := blackstart.InputsToContext(ctx, grantInputs)
	ok, err := m.Check(grantCtx)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, m.Set(grantCtx))

	ok, err = m.Check(grantCtx)
	require.NoError(t, err)
	require.True(t, ok)

	revokeInputs := map[string]blackstart.Input{
		inputConnection: blackstart.NewInputFromValue(db),
		inputRole:       blackstart.NewInputFromValue(grantee),
		inputPermission: blackstart.NewInputFromValue("SELECT"),
		inputScope:      blackstart.NewInputFromValue(string(defaultPrivilegeScopeTables)),
		inputForRole:    blackstart.NewInputFromValue(owner),
		inputSchema:     blackstart.NewInputFromValue("public"),
		inputRevokeMode: blackstart.NewInputFromValue("CASCADE"),
	}
	revokeOp := blackstart.Operation{
		Id:     "default-priv-revoke",
		Module: defaultPrivilegesModuleID,
		Inputs: revokeInputs,
		DoesNotExist: true,
	}
	require.NoError(t, m.Validate(revokeOp))

	revokeCtx := blackstart.InputsToContext(ctx, revokeInputs, blackstart.DoesNotExistFlag)
	ok, err = m.Check(revokeCtx)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, m.Set(revokeCtx))
	ok, err = m.Check(revokeCtx)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestDefaultPrivilegesModule_ValidateRejectsGrantOptionInRevokeMode(t *testing.T) {
	m := newDefaultPrivilegesModule()
	op := blackstart.Operation{
		Id:     "default-priv-invalid",
		Module: defaultPrivilegesModuleID,
		Inputs: map[string]blackstart.Input{
			inputConnection:      blackstart.NewInputFromValue(&fakeConn{}),
			inputRole:            blackstart.NewInputFromValue("app_user"),
			inputPermission:      blackstart.NewInputFromValue("SELECT"),
			inputScope:           blackstart.NewInputFromValue(string(defaultPrivilegeScopeTables)),
			inputWithGrantOption: blackstart.NewInputFromValue(true),
		},
		DoesNotExist: true,
	}
	err := m.Validate(op)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not valid when doesNotExist is true")
}

func TestDefaultPrivilegesModule_ValidateScopeRules(t *testing.T) {
	m := newDefaultPrivilegesModule()

	t.Run("rejects_schema_for_schemas_scope", func(t *testing.T) {
		op := blackstart.Operation{
			Id:     "default-priv-invalid-schema-for-schemas",
			Module: defaultPrivilegesModuleID,
			Inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
				inputRole:       blackstart.NewInputFromValue("app_user"),
				inputPermission: blackstart.NewInputFromValue("USAGE"),
				inputScope:      blackstart.NewInputFromValue("SCHEMAS"),
				inputSchema:     blackstart.NewInputFromValue("public"),
			},
		}
		err := m.Validate(op)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not supported when scope is SCHEMAS")
	})

	t.Run("accepts_large_objects_scope_alias", func(t *testing.T) {
		op := blackstart.Operation{
			Id:     "default-priv-large-objects-alias",
			Module: defaultPrivilegesModuleID,
			Inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
				inputRole:       blackstart.NewInputFromValue("app_user"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("LARGE OBJECTS"),
			},
		}
		err := m.Validate(op)
		require.NoError(t, err)
	})
}

func TestBuildAlterDefaultPrivilegesSQL_NewScopes(t *testing.T) {
	target := defaultPrivilegeTarget{
		Scope:      defaultPrivilegeScopeFunctions,
		OwnerRole:  "admin",
		Grantee:    "PUBLIC",
		Permission: "EXECUTE",
		RevokeMode: "RESTRICT",
	}
	grantQuery := buildAlterDefaultPrivilegesSQL(target, false)
	require.Contains(t, grantQuery, "GRANT EXECUTE ON FUNCTIONS TO PUBLIC;")

	revokeQuery := buildAlterDefaultPrivilegesSQL(target, true)
	require.Contains(t, revokeQuery, "REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC RESTRICT;")
}
