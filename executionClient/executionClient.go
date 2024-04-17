package executionClient

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ExecutionClientComponent struct {
	pulumi.ResourceState
}

type ExecutionClientComponentArgs struct {
	Connection                       *remote.ConnectionArgs
	Client                           string
	Network                          string
	DeploymentType                   string
	DataDir                          string
	ExecutionJwt                     string
	ExecutionClientConfigPath        string
	PodStorageSize                   string
	PodStorageClass                  string
	ExecutionClientImage             string
	ExecutionClientContainerCommands []string
	InstanceNumber                   int
	EnableIngress                    bool
}

const (
	Reth       = "reth"
	Nethermind = "nethermind"
	Geth       = "geth"
	Source     = "source"
	Binary     = "binary"
	Docker     = "docker"
	Kubernetes = "kubernetes"
)

// NewExecutionClientComponent creates a new instance of the ExecutionClientComponent
// and calls the appropriate component constructor based on the client
// being requested.
// It returns a pointer to the ExecutionClientComponent and an error
//
// Example usage:
//
//	client, err := executionClient.NewExecutionClientComponent(ctx, "testRethExecutionClient", &executionClient.ExecutionClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"),
//			Host:       cfg.Require("sshHost"),
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"),
//		},
//		Client:         "reth",
//		Network:        "mainnet",
//		DeploymentType: "source",
//		DataDir:        "/data/mainnet/reth",
//	})
func NewExecutionClientComponent(ctx *pulumi.Context, name string, args *ExecutionClientComponentArgs, opts ...pulumi.ResourceOption) (*ExecutionClientComponent, error) {
	if args == nil {
		args = &ExecutionClientComponentArgs{}
	}

	component := &ExecutionClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:component:ExecutionClient:%s:%s", args.Client, args.Network), name, component, opts...)
	if err != nil {
		return nil, err
	}

	// check what client is being requested and call the appropriate component constructor
	switch args.Client {
	case Reth:
		_, err = NewRethComponent(ctx, "reth", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating reth component", nil)
			return nil, err
		}
	case Nethermind:
		_, err = NewNethermindComponent(ctx, "nethermind", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating nethermind component", nil)
			return nil, err
		}
	case Geth:
		_, err = NewGethComponent(ctx, "geth", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating geth component", nil)
			return nil, err
		}
	}

	return component, nil
}
