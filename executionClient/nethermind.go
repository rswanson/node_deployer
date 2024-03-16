package executionClient

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/rswanson/node_deployer/utils"
)

// NewNethermindComponent creates a new Nethermind execution client component
// and the necessary infrastructure to run it.
//
// Example usage:
//
//	client, err := executionClient.NewNethermindComponent(ctx, "testNethermindExecutionClient", &executionClient.ExecutionClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"), // username for the ssh connection
//			Host:       cfg.Require("sshHost"), // ip address of the host
//			PrivateKey: cfg.RequireSecret("
//		},
//		Client:         "nethermind", // must be "nethermind"
//		Network:        "mainnet", // mainnet, sepolia, or holesky
//		DeploymentType: "source", // source, binary, docker
//		DataDir:        "/data/mainnet/nethermind", // path to the data directory
//	})
func NewNethermindComponent(ctx *pulumi.Context, name string, args *ExecutionClientComponentArgs, opts ...pulumi.ResourceOption) (*ExecutionClientComponent, error) {
	if args == nil {
		args = &ExecutionClientComponentArgs{}
	}

	component := &ExecutionClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:component:ExecutionClient:%s", args.Client), name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Execute a sequence of commands on the remote server
	_, err = remote.NewCommand(ctx, fmt.Sprintf("createDataDir-%s", args.Client), &remote.CommandArgs{
		Create:     pulumi.Sprintf("mkdir -p %s", args.DataDir),
		Connection: args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error creating data directory", nil)
		return nil, err
	}

	if args.DeploymentType == Source {
		// Load configuration
		cfg := config.New(ctx, "")

		// clone repo
		repo, err := remote.NewCommand(ctx, fmt.Sprintf("cloneRepo-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("git clone -b %s %s /data/repos/%s", cfg.Require("nethermindBranch"), cfg.Require("nethermindRepoUrl"), args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error cloning repo", nil)
			return nil, err
		}

		// install dotnet
		dotnetDeps, err := remote.NewCommand(ctx, fmt.Sprintf("installDotnet-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.String("sudo apt update && sudo apt install -y dotnet-sdk-5.0"),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error installing dotnet", nil)
			return nil, err
		}

		// set repo permissions
		repoPerms, err := remote.NewCommand(ctx, fmt.Sprintf("setRepoPermissions-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R %s:%s /data/repos/%s", args.Client, args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo}))
		if err != nil {
			ctx.Log.Error("Error setting repo permissions", nil)
			return nil, err
		}

		// build execution client
		buildClient, err := remote.NewCommand(ctx, fmt.Sprintf("buildExecutionClient-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("cd /data/repos/%s && sudo -u %s dotnet build -c Release", args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{dotnetDeps, repoPerms}))
		if err != nil {
			ctx.Log.Error("Error building execution client", nil)
			return nil, err
		}

		// copy start script
		startScript, err := remote.NewCopyFile(ctx, fmt.Sprintf("copyStartScript-%s", args.Client), &remote.CopyFileArgs{
			LocalPath:  pulumi.Sprintf("scripts/start_%s.sh", args.Client),
			RemotePath: pulumi.Sprintf("/data/scripts/start_%s.sh", args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error copying start script", nil)
			return nil, err
		}

		// script permissions
		scriptPerms, err := remote.NewCommand(ctx, fmt.Sprintf("scriptPermissions-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chmod +x /data/scripts/start_%s.sh", args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{startScript}))
		if err != nil {
			ctx.Log.Error("Error setting script permissions", nil)
			return nil, err
		}

		// create service
		serviceDefinition, err := utils.NewServiceDefinitionComponent(ctx, fmt.Sprintf("executionService-%s", args.Client), &utils.ServiceComponentArgs{
			Connection:  args.Connection,
			ServiceType: args.Client,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{buildClient, scriptPerms}))
		if err != nil {
			ctx.Log.Error("Error creating execution service", nil)
			return nil, err
		}

		// group permissions
		_, err = remote.NewCommand(ctx, fmt.Sprintf("setDataDirGroupPermissions-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R %s:%s %s && chown %s:%s /data/scripts/start_%s.sh", args.Client, args.Client, args.DataDir, args.Client, args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{serviceDefinition, scriptPerms, startScript}))
		if err != nil {
			ctx.Log.Error("Error setting group permissions", nil)
			return nil, err
		}

	}

	return component, nil
}
