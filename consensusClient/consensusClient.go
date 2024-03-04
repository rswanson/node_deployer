package consensusClient

import (
	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ConsensusClientComponent struct {
	Client  string
	Network string
	pulumi.ResourceState
}

type ConsensusClientComponentArgs struct {
	Connection     *remote.ConnectionArgs
	Client         string
	Network        string
	DeploymentType string
	DataDir        string
}

const (
	Teku       = "teku"
	Prysm      = "prysm"
	Lighthouse = "lighthouse"
	Lodestar   = "lodestar"
	Nimbus     = "nimbus"
	Source     = "source"
	Binary     = "binary"
)

func NewConsensusClientComponent(ctx *pulumi.Context, name string, args *ConsensusClientComponentArgs, opts ...pulumi.ResourceOption) (*ConsensusClientComponent, error) {
	if args == nil {
		args = &ConsensusClientComponentArgs{}
	}

	component := &ConsensusClientComponent{}
	err := ctx.RegisterComponentResource("custom:componenet:ConsensusClient", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// set the client and network
	component.Client = args.Client
	component.Network = args.Network

	switch component.Client {
	case Teku:
		_, err = NewTekuComponent(ctx, "teku", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating teku component", nil)
			return nil, err
		}
	case Prysm:
		_, err = NewPrysmComponent(ctx, "prysm", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating prysm component", nil)
			return nil, err
		}
	case Lighthouse:
		_, err = NewLighthouseComponent(ctx, "lighthouse", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating lighthouse component", nil)
			return nil, err
		}
	case Lodestar:
		_, err = NewLodestarComponent(ctx, "lodestar", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating lodestar component", nil)
			return nil, err
		}
	case Nimbus:
		_, err = NewNimbusComponent(ctx, "nimbus", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating nimbus component", nil)
			return nil, err
		}
	}

	return component, nil
}
