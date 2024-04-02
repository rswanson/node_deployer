package executionClient

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/rswanson/node_deployer/utils"
)

// NewRethComponent creates a new reth execution client component
// and the necessary infrastructure to run it.
//
// Example usage:
//
//	client, err := executionClient.NewRethComponent(ctx, "testRethExecutionClient", &executionClient.ExecutionClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"), // username for the ssh connection
//			Host:       cfg.Require("sshHost"), // ip address of the host
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"), // must be a secret, RequireSecret is critical for security
//		},
//		Client:         "reth", // must be "reth"
//		Network:        "mainnet", // mainnet, sepolia, or holesky
//		DeploymentType: "source", // source, binary, docker
//		DataDir:        "/data/mainnet/reth", // path to the data directory
//	})
func NewRethComponent(ctx *pulumi.Context, name string, args *ExecutionClientComponentArgs, opts ...pulumi.ResourceOption) (*ExecutionClientComponent, error) {
	if args == nil {
		args = &ExecutionClientComponentArgs{}
	}

	component := &ExecutionClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:component:ExecutionClient:%s", args.Client), name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Execute a sequence of commands on the remote server
	_, err = remote.NewCommand(ctx, fmt.Sprintf("createDataDir-%s", args.Network), &remote.CommandArgs{
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

		// copy start script
		startScript, err := remote.NewCopyFile(ctx, fmt.Sprintf("copyStartScript-%s", args.Network), &remote.CopyFileArgs{
			LocalPath:  pulumi.Sprintf("scripts/start_%s_%s.sh", args.Client, args.Network),
			RemotePath: pulumi.Sprintf("/data/scripts/start_%s_%s.sh", args.Client, args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error copying start script", nil)
			return nil, err
		}

		// script permissions
		_, err = remote.NewCommand(ctx, fmt.Sprintf("scriptPermissions-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chmod +x /data/scripts/start_%s_%s.sh", args.Client, args.Network),
			Delete:     pulumi.String("echo 0"),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{startScript}))
		if err != nil {
			ctx.Log.Error("Error setting script permissions", nil)
			return nil, err
		}

		// Execute a sequence of commands on the remote serve`r
		repo, err := remote.NewCommand(ctx, fmt.Sprintf("cloneRepo-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("git clone -b %s %s /data/repos/%s/reth", cfg.Require("rethGitBranch"), cfg.Require("rethRepoURL"), args.Network),
			Update:     pulumi.String("cd /data/repos/reth && git pull"),
			Delete:     pulumi.Sprintf("rm -rf /data/repos/%s/reth", args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error cloning repo", nil)
			return nil, err
		}

		// set group permissions
		ownership, err := remote.NewCommand(ctx, fmt.Sprintf("setGroupPermissions-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R reth:reth /data/repos/%s/reth", args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, startScript}))
		if err != nil {
			ctx.Log.Error("Error setting group permissions", nil)
			return nil, err
		}

		// install rust toolchain
		rustToolchain, err := remote.NewCommand(ctx, fmt.Sprintf("installRust-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.String("sudo -u reth curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sudo -u reth sh -s -- -y"),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error installing rust toolchain", nil)
			return nil, err
		}
		rethInstallation := &remote.Command{}
		if args.Network == "base" {
			rethInstallation, err = remote.NewCommand(ctx, fmt.Sprintf("installReth-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/reth/bin/reth --bin op-reth --features \"optimism\" --root /data", args.Connection.User, args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain, ownership}))
			if err != nil {
				ctx.Log.Error("Error installing reth", nil)
				return nil, err
			}
		} else if args.Network == "sepolia" {
			rethInstallation, err = remote.NewCommand(ctx, fmt.Sprintf("installReth-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/reth/bin/reth --bin reth", args.Connection.User, args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain, ownership}))
			if err != nil {
				ctx.Log.Error("Error installing reth", nil)
				return nil, err
			}
			_, err := remote.NewCommand(ctx, fmt.Sprintf("moveAndRename-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("mv /root/.cargo/bin/reth /data/bin/reth-%s", args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{rethInstallation}))
			if err != nil {
				ctx.Log.Error("Error moving and renaming reth", nil)
				return nil, err
			}
		} else if args.Network == "holesky" {
			rethInstallation, err = remote.NewCommand(ctx, fmt.Sprintf("installReth-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/reth/bin/reth --bin reth", args.Connection.User, args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain, ownership}))
			if err != nil {
				ctx.Log.Error("Error installing reth", nil)
				return nil, err
			}
			_, err := remote.NewCommand(ctx, fmt.Sprintf("moveAndRename-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("mv /root/.cargo/bin/reth /data/bin/reth-%s", args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{rethInstallation}))
			if err != nil {
				ctx.Log.Error("Error moving and renaming reth", nil)
				return nil, err
			}
		} else {

			rethInstallation, err = remote.NewCommand(ctx, fmt.Sprintf("installReth-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/reth/bin/reth --bin reth --root /data", args.Connection.User, args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain, ownership}))
			if err != nil {
				ctx.Log.Error("Error installing reth", nil)
				return nil, err
			}
		}

		// group permissions
		groupPerms, err := remote.NewCommand(ctx, fmt.Sprintf("setDataDirGroupPermissions-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R %s:%s %s && chown %s:%s /data/bin/%s && chown %s:%s /data/scripts/start_%s_%s.sh", args.Client, args.Client, args.DataDir, args.Client, args.Client, args.Client, args.Client, args.Client, args.Client, args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, startScript, rethInstallation}))
		if err != nil {
			ctx.Log.Error("Error setting group permissions", nil)
			return nil, err
		}

		if args.Network == "base" {
			_, err = utils.NewServiceDefinitionComponent(ctx, fmt.Sprintf("rethBaseService-%s", args.Network), &utils.ServiceComponentArgs{
				Connection:  args.Connection,
				ServiceType: args.Network,
				Network:     args.Network,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{groupPerms, rethInstallation}))
			if err != nil {
				ctx.Log.Error("Error creating reth service", nil)
				return nil, err
			}
		} else {
			_, err = utils.NewServiceDefinitionComponent(ctx, fmt.Sprintf("rethService-%s", args.Network), &utils.ServiceComponentArgs{
				Connection:  args.Connection,
				ServiceType: args.Client,
				Network:     args.Network,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{groupPerms, rethInstallation}))
			if err != nil {
				ctx.Log.Error("Error creating reth service", nil)
				return nil, err
			}
		}
	} else if args.DeploymentType == Binary {
		ctx.Log.Error("Binary deployment type not yet supported", nil)
	} else if args.DeploymentType == Docker {
		ctx.Log.Error("Docker deployment type not yet supported", nil)
	}

	return component, nil
}
