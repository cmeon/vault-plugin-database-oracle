package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/tgulacsi/go/orahlp"

	"github.com/hashicorp/vault/sdk/database/dbplugin"
	dockertest "gopkg.in/ory-am/dockertest.v3"
)

const (
	defaultUser     = "system"
	defaultPassword = "oracle"
)

func prepareOracleTestContainer(t *testing.T) (connString string, cleanup func()) {
	if os.Getenv("ORACLE_DSN") != "" {
		return os.Getenv("ORACLE_DSN"), func() {}
	}

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Failed to connect to docker: %s", err)
	}

	resource, err := pool.Run("wnameless/oracle-xe-11g-r2", "latest", []string{})
	if err != nil {
		t.Fatalf("Could not start local Oracle docker container: %s", err)
	}

	cleanup = func() {
		err := pool.Purge(resource)
		if err != nil {
			t.Fatalf("Failed to cleanup local container: %s", err)
		}
	}

	connString = fmt.Sprintf("%s/%s@localhost:%s/xe", defaultUser, defaultPassword, resource.GetPort("1521/tcp"))

	// exponential backoff-retry
	// the oracle container seems to take at least one minute to start, give us two
	pool.MaxWait = time.Minute * 2
	if err = pool.Retry(func() error {
		var err error
		var db *sql.DB
		db, err = sql.Open("oci8", connString)
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		t.Fatalf("Could not connect to Oracle docker container: %s", err)
	}

	return connString, cleanup
}

func TestOracle_Initialize(t *testing.T) {
	connURL, cleanup := prepareOracleTestContainer(t)
	defer cleanup()

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
	}

	db := new()

	err := db.Initialize(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	connProducer := db.SQLConnectionProducer
	if !connProducer.Initialized {
		t.Fatal("Database should be initalized")
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestOracle_CreateUser(t *testing.T) {
	connURL, cleanup := prepareOracleTestContainer(t)
	defer cleanup()

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
	}

	db := new()
	err := db.Initialize(context.Background(), connectionDetails, true)

	if err != nil {
		t.Fatalf("err: %s", err)
	}

	usernameConfig := dbplugin.UsernameConfig{
		DisplayName: "test",
		RoleName:    "test",
	}

	// Test with no configured Creation Statement
	_, _, err = db.CreateUser(context.Background(), dbplugin.Statements{}, usernameConfig, time.Now().Add(time.Minute))
	if err == nil {
		t.Fatal("Expected error when no creation statement is provided")
	}

	statements := dbplugin.Statements{
		CreationStatements: creationStatements,
	}

	username, password, err := db.CreateUser(context.Background(), statements, usernameConfig, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err = testCredentialsExist(connURL, username, password); err != nil {
		t.Fatalf("Could not connect with new credentials: %s", err)
	}
}

func TestOracle_RenewUser(t *testing.T) {
	connURL, cleanup := prepareOracleTestContainer(t)
	defer cleanup()

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
	}

	db := new()

	err := db.Initialize(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	usernameConfig := dbplugin.UsernameConfig{
		DisplayName: "test",
		RoleName:    "test",
	}

	statements := dbplugin.Statements{
		CreationStatements: creationStatements,
	}

	username, password, err := db.CreateUser(context.Background(), statements, usernameConfig, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err = testCredentialsExist(connURL, username, password); err != nil {
		t.Fatalf("Could not connect with new credentials: %s", err)
	}

	err = db.RenewUser(context.Background(), statements, username, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Sleep longer than the initial expiration time
	time.Sleep(2 * time.Second)

	if err = testCredentialsExist(connURL, username, password); err != nil {
		t.Fatalf("Could not connect with new credentials: %s", err)
	}
}

func TestOracle_RevokeUser(t *testing.T) {
	connURL, cleanup := prepareOracleTestContainer(t)
	defer cleanup()

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
	}

	db := new()

	err := db.Initialize(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	usernameConfig := dbplugin.UsernameConfig{
		DisplayName: "test",
		RoleName:    "test",
	}

	statements := dbplugin.Statements{
		CreationStatements: creationStatements,
	}

	username, password, err := db.CreateUser(context.Background(), statements, usernameConfig, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err = testCredentialsExist(connURL, username, password); err != nil {
		t.Fatalf("Could not connect with new credentials: %s", err)
	}

	err = db.RevokeUser(context.Background(), statements, username)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := testCredentialsExist(connURL, username, password); err == nil {
		t.Fatal("Credentials were not revoked")
	}
}

func TestOracle_RevokeUserWithCustomStatements(t *testing.T) {
	connURL, cleanup := prepareOracleTestContainer(t)
	defer cleanup()

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
	}

	db := new()

	err := db.Initialize(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	usernameConfig := dbplugin.UsernameConfig{
		DisplayName: "test",
		RoleName:    "test",
	}

	statements := dbplugin.Statements{
		CreationStatements: creationStatements,
	}

	username, password, err := db.CreateUser(context.Background(), statements, usernameConfig, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err = testCredentialsExist(connURL, username, password); err != nil {
		t.Fatalf("Could not connect with new credentials: %s", err)
	}

	statements.RevocationStatements = `
REVOKE CONNECT FROM {{name}};
REVOKE CREATE SESSION FROM {{name}};
DROP USER {{name}};
`
	err = db.RevokeUser(context.Background(), statements, username)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := testCredentialsExist(connURL, username, password); err == nil {
		t.Fatal("Credentials were not revoked")
	}
}

func TestOracle_RotateRootCredentials(t *testing.T) {
	testRotateRootCredentialsCore(t, false)
	testRotateRootCredentialsCore(t, true)
}

func testRotateRootCredentialsCore(t *testing.T, custom bool) {
	connURL, cleanup := prepareOracleTestContainer(t)
	defer cleanup()

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
		"username":       defaultUser,
		"password":       defaultPassword,
	}

	db := new()

	err := db.Initialize(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = testCredentialsExist(connURL, defaultUser, defaultPassword)
	if err != nil {
		t.Fatalf("unable to connect with original credentials: %v", err)
	}

	var rotateStatement []string
	if custom {
		rotateStatement = []string{`alter user {{username}} identified by {{password}}`}
	}
	newConf, err := db.RotateRootCredentials(context.Background(), rotateStatement)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if newConf["password"].(string) == defaultPassword {
		t.Fatal("password was not updated")
	}

	err = testCredentialsExist(connURL, defaultUser, newConf["password"].(string))
	if err != nil {
		t.Fatalf("unable to connect with new credentials: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func testCredentialsExist(connString, username, password string) error {
	// Log in with the new credentials
	_, _, link := orahlp.SplitDSN(connString)
	connURL := fmt.Sprintf("%s/%s@%s", username, password, link)

	db, err := sql.Open("oci8", connURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}

const creationStatements = `
CREATE USER {{name}} IDENTIFIED BY {{password}};
GRANT CONNECT TO {{name}};
GRANT CREATE SESSION TO {{name}};
`

func TestSplitQueries(t *testing.T) {
	type testCase struct {
		input    []string
		expected []string
	}

	tests := map[string]testCase{
		"nil input": {
			input:    nil,
			expected: nil,
		},
		"empty input": {
			input:    []string{},
			expected: nil,
		},
		"empty string": {
			input:    []string{""},
			expected: nil,
		},
		"string with only semicolon": {
			input:    []string{";"},
			expected: nil,
		},
		"only semicolons": {
			input:    []string{";;;;"},
			expected: nil,
		},
		"single input": {
			input: []string{
				"alter user {{username}} identified by {{password}}",
			},
			expected: []string{
				"alter user {{username}} identified by {{password}}",
			},
		},
		"single input with trailing semicolon": {
			input: []string{
				"alter user {{username}} identified by {{password}};",
			},
			expected: []string{
				"alter user {{username}} identified by {{password}}",
			},
		},
		"single input with leading semicolon": {
			input: []string{
				";alter user {{username}} identified by {{password}}",
			},
			expected: []string{
				"alter user {{username}} identified by {{password}}",
			},
		},
		"multiple queries in single line": {
			input: []string{
				"alter user {{username}} identified by {{password}};do something with {{username}} {{password}};",
			},
			expected: []string{
				"alter user {{username}} identified by {{password}}",
				"do something with {{username}} {{password}}",
			},
		},
		"multiple queries in multiple lines": {
			input: []string{
				"foo;bar;baz",
				"qux ; quux ; quuz",
			},
			expected: []string{
				"foo",
				"bar",
				"baz",
				"qux",
				"quux",
				"quuz",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := splitQueries(test.input)

			if !reflect.DeepEqual(actual, test.expected) {
				t.FailNow()
			}
		})
	}
}

func TestSetCredentials_missingArguments(t *testing.T) {
	type testCase struct {
		userConfig dbplugin.StaticUserConfig
	}

	tests := map[string]testCase{
		"missing username": {
			dbplugin.StaticUserConfig{
				Username: "",
				Password: "newpassword",
			},
		},
		"missing password": {
			dbplugin.StaticUserConfig{
				Username: "testuser",
				Password: "",
			},
		},
		"missing username and password": {
			dbplugin.StaticUserConfig{
				Username: "",
				Password: "",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			connURL, cleanup := prepareOracleTestContainer(t)
			defer cleanup()

			connectionDetails := map[string]interface{}{
				"connection_url": connURL,
			}

			db := new()
			err := db.Initialize(context.Background(), connectionDetails, true)
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			// Create a context with a timeout so we don't spin forever in a worst case
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			updatedUser, updatedPass, err := db.SetCredentials(ctx, dbplugin.Statements{}, test.userConfig)
			if err == nil {
				t.Fatalf("error expected, got nil")
			}
			if updatedUser != "" {
				t.Fatalf("username provided when it should have errored: %s", updatedUser)
			}
			if updatedPass != "" {
				t.Fatalf("new password provided when it should have errored: %s", updatedPass)
			}
		})
	}
}

func TestSetCredentials_rotationStatements(t *testing.T) {
	type testCase struct {
		rotationStatements []string
	}

	tests := map[string]testCase{
		"no rotation statements": {
			rotationStatements: []string{},
		},
		"explicit default": {
			rotationStatements: []string{`ALTER USER "{{username}}" IDENTIFIED BY "{{password}}"`},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			connURL, cleanup := prepareOracleTestContainer(t)
			defer cleanup()

			connectionDetails := map[string]interface{}{
				"connection_url": connURL,
			}

			db := new()
			err := db.Initialize(context.Background(), connectionDetails, true)
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			usernameConfig := dbplugin.UsernameConfig{
				DisplayName: "testuser",
				RoleName:    "testrole",
			}

			statements := dbplugin.Statements{
				Creation: []string{creationStatements},
				Rotation: test.rotationStatements,
			}

			// Create a context with a timeout so we don't spin forever in a worst case
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			createdUser, firstPass, err := db.CreateUser(ctx, statements, usernameConfig, time.Now().Add(time.Minute))
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			updatedUserConfig := dbplugin.StaticUserConfig{
				Username: createdUser,
				Password: "newpassword",
			}

			updatedUser, updatedPass, err := db.SetCredentials(ctx, statements, updatedUserConfig)
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			if updatedUser != createdUser {
				t.Fatalf("username changed %s => %s", createdUser, updatedUser)
			}

			if updatedPass == firstPass {
				t.Fatalf("password did not change")
			}

			if updatedPass != updatedUserConfig.Password {
				t.Fatalf("password changed to the wrong password")
			}

			if err = testCredentialsExist(connURL, updatedUser, updatedPass); err != nil {
				t.Fatalf("Could not connect with updated credentials: %s", err)
			}
		})
	}
}
