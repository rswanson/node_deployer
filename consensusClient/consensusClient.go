package consensusClient

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ConsensusClientComponent struct {
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
	err := ctx.RegisterComponentResource(fmt.Sprintf("reth:consensus:%s", args.Client), name, component, opts...)
	if err != nil {
		return nil, err
	}

	switch args.Client {
	case Teku:
		_, err = NewTekuComponent(ctx, "teku", args, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating teku component", nil)
			return nil, err
		}
	case Prysm:
		ctx.Log.Error("Prysm client not yet supported", nil)
	case Lighthouse:
		ctx.Log.Error("Lighthouse client not yet supported", nil)
	case Lodestar:
		ctx.Log.Error("Lodestar client not yet supported", nil)
	case Nimbus:
		ctx.Log.Error("Nimbus client not yet supported", nil)
	}

	return component, nil
}
