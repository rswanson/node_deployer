package node_deployer

import (
	"fmt"
	"os"

	"github.com/rswanson/node_deployer/consensusClient"
	"github.com/rswanson/node_deployer/executionClient"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type DeploymentComponent struct {
	pulumi.ResourceState
	Connection      *remote.ConnectionArgs
	Network         string
	DeploymentType  string
	ConsensusClient string
	ExecutionClient string
}

type DeploymentComponentArgs struct {
	Connection      *remote.ConnectionArgs
	Network         string
	DeploymentType  string
	ConsensusClient string
	ExecutionClient string
}

const (
	// list of clients
	Reth       string = "reth"
	Geth       string = "geth"
	Nethermind string = "nethermind"
	Lighthouse string = "lighthouse"
	Lodestar   string = "lodestar"
	Nimbus     string = "nimbus"
	Prysm      string = "prysm"
	Teku       string = "teku"

	// list of networks
	Mainnet string = "mainnet"
	Sepolia string = "sepolia"
	Holesky string = "holesky"

	// list of deployment types
	Source string = "source"
	Binary string = "binary"
	Docker string = "docker"
	DryRun string = "dry-run"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Load configuration
		cfg := config.New(ctx, "")

		// SSH Key and Server Details
		sshPrivateKey := cfg.RequireSecret("sshPrivateKey")
		serverIP := cfg.Require("serverIP")
		serverUser := cfg.Require("serverUser")

		connection := &remote.ConnectionArgs{
			Host:       pulumi.String(serverIP),
			User:       pulumi.String(serverUser),
			PrivateKey: sshPrivateKey,
		}

		// Read deploy type from system environment variables
		deployType := os.Getenv("DEPLOY_TYPE")
		if deployType == Source {
			_, err := NewDeploymentComponent(ctx, "deploymentFromSource", &DeploymentComponentArgs{
				Connection:      connection,
				Network:         Mainnet,
				DeploymentType:  Source,
				ConsensusClient: Teku,
				ExecutionClient: Reth,
			})
			if err != nil {
				return err
			}
			_, err = NewDeploymentComponent(ctx, "deploymentFromSource", &DeploymentComponentArgs{
				Connection:      connection,
				Network:         Mainnet,
				DeploymentType:  Source,
				ConsensusClient: Prysm,
				ExecutionClient: Reth,
			})
			if err != nil {
				return err
			}
		} else if deployType == Binary {
			ctx.Log.Error("Binary deployment type not yet supported", nil)
		} else if deployType == Docker {
			ctx.Log.Error("Docker deployment type not yet supported", nil)
			// return deployFromDocker(ctx, *connection)
		} else if deployType == DryRun {
			if deployType == DryRun {
				_, err := remote.NewCommand(ctx, "dryRun", &remote.CommandArgs{
					Create:     pulumi.String("echo 0"),
					Delete:     pulumi.String("echo 0"),
					Connection: connection,
				})
				if err != nil {
					ctx.Log.Error("Error running dry run", nil)
					return nil
				}
			}
		}

		return nil
	})
}

func NewDeploymentComponent(ctx *pulumi.Context, name string, args *DeploymentComponentArgs, opts ...pulumi.ResourceOption) (*DeploymentComponent, error) {
	if args == nil {
		args = &DeploymentComponentArgs{}
	}

	component := &DeploymentComponent{}
	err := ctx.RegisterComponentResource("custom:resource:NodeDeployment", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// create the shared directory and jwt secret
	sharedDataDir, err := remote.NewCommand(ctx, "createDataDir", &remote.CommandArgs{
		Create:     pulumi.String("mkdir -p /data/shared"),
		Connection: args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error creating data/shared directory", nil)
		return nil, err
	}

	// create the bin directory
	_, err = remote.NewCommand(ctx, "createBinDir", &remote.CommandArgs{
		Create:     pulumi.String("mkdir -p /data/bin"),
		Connection: args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error creating bin directory", nil)
		return nil, err
	}

	// get jwt secret from config
	jwt := config.New(ctx, "").RequireSecret("jwtSecret")
	jwtFile, err := remote.NewCommand(ctx, "createJwtSecret", &remote.CommandArgs{
		Create:     pulumi.Sprintf("echo %s > /data/shared/jwt.hex", jwt),
		Connection: args.Connection,
	}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{sharedDataDir}))
	if err != nil {
		ctx.Log.Error("Error creating jwt secret", nil)
		return nil, err
	}

	// set group permissions
	_, err = remote.NewCommand(ctx, "setGroupPermissions", &remote.CommandArgs{
		Create:     pulumi.String("chown -R reth:eth /data/shared"),
		Connection: args.Connection,
	}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{jwtFile}))
	if err != nil {
		ctx.Log.Error("Error setting group permissions", nil)
		return nil, err
	}

	// create scripts directory
	scriptsDir, err := remote.NewCommand(ctx, "createScriptsDir", &remote.CommandArgs{
		Create:     pulumi.String("mkdir -p /data/scripts"),
		Connection: args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error creating scripts directory", nil)
		return nil, err
	}

	// create repos dir
	_, err = remote.NewCommand(ctx, "createReposDir", &remote.CommandArgs{
		Create:     pulumi.String("mkdir -p /data/repos"),
		Connection: args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error creating repos directory", nil)
		return nil, err
	}

	_, err = consensusClient.NewConsensusClientComponent(ctx, "consensusClient", &consensusClient.ConsensusClientComponentArgs{
		Client:         args.ConsensusClient,
		Network:        args.Network,
		DeploymentType: args.DeploymentType,
		DataDir:        fmt.Sprintf("/data/%s/%s", args.Network, args.ConsensusClient),
		Connection:     args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error creating consensus client", nil)
		return nil, err
	}

	// Create execution client
	_, err = executionClient.NewExecutionClientComponent(ctx, "executionClient", &executionClient.ExecutionClientComponentArgs{
		Client:         args.ExecutionClient,
		Network:        args.Network,
		DeploymentType: args.DeploymentType,
		DataDir:        fmt.Sprintf("/data/%s/%s", args.Network, args.ExecutionClient),
		Connection:     args.Connection,
	}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{sharedDataDir, scriptsDir}))
	if err != nil {
		ctx.Log.Error("Error creating execution client", nil)
		return nil, err
	}

	return component, nil
}
