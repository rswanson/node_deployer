package consensusClient

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/rswanson/node-deployer/utils"
)

// NewNimbusComponent creates a new consensus client component for Nimbus
// and returns a pointer to the component
//
// Example usage:
//
//	client, err := consensusClient.NewNimbusComponent(ctx, "testNimbusConsensusClient", &consensusClient.ConsensusClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"),             // username for the ssh connection
//			Host:       cfg.Require("sshHost"),             // ip address of the host
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"), // must be a secret, RequireSecret is critical for security
//		},
//		Client:         "nimbus",             // must be "nimbus"
//		Network:        "mainnet",            // mainnet, sepolia, or holesky
//		DeploymentType: "source",             // source, binary, docker
//		DataDir:        "/data/nimbus",	  // path to the data directory
//	})
func NewNimbusComponent(ctx *pulumi.Context, name string, args *ConsensusClientComponentArgs, opts ...pulumi.ResourceOption) (*ConsensusClientComponent, error) {
	if args == nil {
		args = &ConsensusClientComponentArgs{}
	}

	component := &ConsensusClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:componenet:ConsensusClient:%s", args.Client), name, component, opts...)
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
			Create:     pulumi.Sprintf("git clone -b %s %s /data/repos/%s", cfg.Require("nimbusBranch"), cfg.Require("nimbusRepoUrl"), args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error cloning repo", nil)
			return nil, err
		}

		// install pre-reqs
		preReqs, err := remote.NewCommand(ctx, fmt.Sprintf("installPrereqs-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.String("sudo apt install -y git cmake build-essential"),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error installing pre-reqs", nil)
			return nil, err
		}

		// build consensus client
		buildClient, err := remote.NewCommand(ctx, fmt.Sprintf("buildConsensusClient-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("cd /data/repos/%s && sudo -u %s make -j4 nimbus_beacon_node", args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, preReqs}))
		if err != nil {
			ctx.Log.Error("Error building consensus client", nil)
			return nil, err
		}

		// copy binary to /usr/local/bin
		_, err = remote.NewCommand(ctx, fmt.Sprintf("moveClientBinary-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("mv /data/repos/%s/build/nimbus_beacon_node /usr/local/bin/nimbus_beacon_node", args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{buildClient}))
		if err != nil {
			ctx.Log.Error("Error moving client binary", nil)
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
		serviceDefinition, err := utils.NewServiceDefinitionComponent(ctx, fmt.Sprintf("consensusService-%s", args.Client), &utils.ServiceComponentArgs{
			Connection:  args.Connection,
			ServiceType: args.Client,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{buildClient, scriptPerms}))
		if err != nil {
			ctx.Log.Error("Error creating consensus service", nil)
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
