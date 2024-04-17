package consensusClient_test

import (
	"testing"

	"github.com/rswanson/node_deployer/consensusClient"

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

func TestConsensusClientComponent(t *testing.T) {
	t.Run("TekuComponent", func(t *testing.T) {
		mocks := mocks(0)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// Create a new instance of the ConsensusClientComponent
			_, err := consensusClient.NewConsensusClientComponent(ctx, "testTekuConsensusClient", &consensusClient.ConsensusClientComponentArgs{
				Connection:     &remote.ConnectionArgs{},
				Client:         "teku",
				Network:        "testNetwork",
				DeploymentType: "testDeploymentType",
				DataDir:        "testDataDir",
			})

			// Test the NewTekuComponent function

			assert.NoError(t, err, "Expected to not receive an error")

			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err, "Expected to not receive an error")
	})

	t.Run("PrysmComponent", func(t *testing.T) {
		mocks := mocks(0)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// Create a new instance of the ConsensusClientComponent
			_, err := consensusClient.NewConsensusClientComponent(ctx, "testPrysmConsensusClient", &consensusClient.ConsensusClientComponentArgs{
				Connection:     &remote.ConnectionArgs{},
				Client:         "prysm",
				Network:        "testNetwork",
				DeploymentType: "testDeploymentType",
				DataDir:        "testDataDir",
			})

			assert.NoError(t, err, "Expected to not receive an error")

			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err, "Expected to not receive an error")
	})

	t.Run("LighthouseComponent", func(t *testing.T) {
		mocks := mocks(0)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// Create a new instance of the ConsensusClientComponent
			_, err := consensusClient.NewConsensusClientComponent(ctx, "testLighthouseConsensusClient", &consensusClient.ConsensusClientComponentArgs{
				Connection:     &remote.ConnectionArgs{},
				Client:         "lighthouse",
				Network:        "testNetwork",
				DeploymentType: "testDeploymentType",
				DataDir:        "testDataDir",
			})

			assert.NoError(t, err, "Expected to not receive an error")

			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err, "Expected to not receive an error")
	})

	t.Run("LodestarComponent", func(t *testing.T) {
		mocks := mocks(0)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// Create a new instance of the ConsensusClientComponent
			_, err := consensusClient.NewConsensusClientComponent(ctx, "testLodestarConsensusClient", &consensusClient.ConsensusClientComponentArgs{
				Connection:     &remote.ConnectionArgs{},
				Client:         "lodestar",
				Network:        "testNetwork",
				DeploymentType: "testDeploymentType",
				DataDir:        "testDataDir",
			})

			assert.NoError(t, err, "Expected to not receive an error")

			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err, "Expected to not receive an error")
	})

	t.Run("NimbusComponent", func(t *testing.T) {
		mocks := mocks(0)
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// Create a new instance of the ConsensusClientComponent
			_, err := consensusClient.NewConsensusClientComponent(ctx, "testNimbusConsensusClient", &consensusClient.ConsensusClientComponentArgs{
				Connection:     &remote.ConnectionArgs{},
				Client:         "nimbus",
				Network:        "testNetwork",
				DeploymentType: "testDeploymentType",
				DataDir:        "testDataDir",
			})

			assert.NoError(t, err, "Expected to not receive an error")

			return nil
		}, pulumi.WithMocks("project", "stack", mocks))
		assert.NoError(t, err, "Expected to not receive an error")
	})

}
