package executionClient

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ExecutionClientComponent struct {
	pulumi.ResourceState
	Client  string
	Network string
}

type ExecutionClientComponentArgs struct {
	Connection     *remote.ConnectionArgs
	Client         string
	Network        string
	DeploymentType string
	DataDir        string
}

const (
	Reth       = "reth"
	Nethermind = "nethermind"
	Geth       = "geth"
	Source     = "source"
	Binary     = "binary"
	Docker     = "docker"
)

func NewExecutionClientComponent(ctx *pulumi.Context, name string, args *ExecutionClientComponentArgs, opts ...pulumi.ResourceOption) (*ExecutionClientComponent, error) {
	if args == nil {
		args = &ExecutionClientComponentArgs{}
	}

	component := &ExecutionClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("reth:execution:%s", args.Client), name, component, opts...)
	if err != nil {
		return nil, err
	}

	// set the client and network
	component.Client = args.Client
	component.Network = args.Network

	// check what client is being requested and call the appropriate component constructor
	switch component.Client {
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
