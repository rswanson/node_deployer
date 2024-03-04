package executionClient_test

import (
	"testing"

	el "github.com/rswanson/node_deployer/executionClient"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func TestExecutionClientComponent(t *testing.T) {
	t.Run("RethComponent", func(t *testing.T) {
		mocks := mocks(0)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// Create a new instance of the ExecutionClientComponent
			el_client, err := el.NewExecutionClientComponent(ctx, "testRethExecutionClient", &el.ExecutionClientComponentArgs{
				Connection:     &remote.ConnectionArgs{},
				Client:         "reth",
				Network:        "testNetwork",
				DeploymentType: "testDeploymentType",
				DataDir:        "testDataDir",
			})

			// Test the NewRethComponent function

			assert.NoError(t, err, "Expected to not receive an error")

			// Test that the client is reth
			assert.Equal(t, "reth", el_client.Client, "Expected client to be reth, but got %s", el_client.Client)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err, "Expected to not receive an error")
	})

	t.Run("NethermindComponent", func(t *testing.T) {
		mocks := mocks(0)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// Create a new instance of the ExecutionClientComponent
			client, err := el.NewExecutionClientComponent(ctx, "testNethermindExecutionClient", &el.ExecutionClientComponentArgs{
				Connection:     &remote.ConnectionArgs{},
				Client:         "nethermind",
				Network:        "testNetwork",
				DeploymentType: "testDeploymentType",
				DataDir:        "testDataDir",
			})

			// Test the NewNethermindComponent function

			assert.NoError(t, err, "Expected to not receive an error")

			// Test that the client is nethermind
			assert.Equal(t, "nethermind", client.Client, "Expected client to be nethermind, but got %s", client.Client)

			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err, "Expected to not receive an error")
	})

	t.Run("GethComponent", func(t *testing.T) {
		mocks := mocks(0)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// Create a new instance of the ExecutionClientComponent
			client, err := el.NewExecutionClientComponent(ctx, "testGethExecutionClient", &el.ExecutionClientComponentArgs{
				Connection:     &remote.ConnectionArgs{},
				Client:         "geth",
				Network:        "testNetwork",
				DeploymentType: "testDeploymentType",
				DataDir:        "testDataDir",
			})

			// Test the NewGethComponent function

			assert.NoError(t, err, "Expected to not receive an error")

			// Test that the client is geth
			assert.Equal(t, "geth", client.Client, "Expected client to be geth, but got %s", client.Client)

			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err, "Expected to not receive an error")
	})

}

func TestExecutionClientComponentArgs(t *testing.T) {
	connection := &remote.ConnectionArgs{
		// Initialize connection args here
	}

	client := "testClient"
	network := "testNetwork"
	deploymentType := "testDeploymentType"
	dataDir := "testDataDir"

	args := el.ExecutionClientComponentArgs{
		Connection:     connection,
		Client:         client,
		Network:        network,
		DeploymentType: deploymentType,
		DataDir:        dataDir,
	}

	// Test Connection field
	assert.Equal(t, connection, args.Connection, "Expected Connection to be %v, but got %v", connection, args.Connection)

	// Test Client field
	if args.Client != client {
		t.Errorf("Expected Client to be %s, but got %s", client, args.Client)
	}

	// Test Network field
	if args.Network != network {
		t.Errorf("Expected Network to be %s, but got %s", network, args.Network)
	}

	// Test DeploymentType field
	if args.DeploymentType != deploymentType {
		t.Errorf("Expected DeploymentType to be %s, but got %s", deploymentType, args.DeploymentType)
	}

	// Test DataDir field
	if args.DataDir != dataDir {
		t.Errorf("Expected DataDir to be %s, but got %s", dataDir, args.DataDir)
	}
}
