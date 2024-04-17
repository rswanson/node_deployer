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
	Connection                       *remote.ConnectionArgs
	Client                           string
	Network                          string
	DeploymentType                   string
	DataDir                          string
	ConsensusClientConfigPath        string
	ConsensusClientImage             string
	ConsensusClientContainerCommands []string
	InstanceNumber                   int
	EnableRpcIngress                 bool
	PodStorageClass                  string
	PodStorageSize                   string
	ExecutionJwt                     string
}

const (
	Teku       = "teku"
	Prysm      = "prysm"
	Lighthouse = "lighthouse"
	Lodestar   = "lodestar"
	Nimbus     = "nimbus"
	Source     = "source"
	Binary     = "binary"
	Docker     = "docker"
	Kubernetes = "kubernetes"
)

// NewConsensusClientComponent creates a new instance of the ConsensusClientComponent
// and calls the appropriate component constructor based on the client
// being requested.
// It returns a pointer to the ConsensusClientComponent and an error
//
// Example usage:
//
//	client, err := consensusClient.NewConsensusClientComponent(ctx, "testLighthouseConsensusClient", &consensusClient.ConsensusClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"),
//			Host:       cfg.Require("sshHost"),
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"),
//		},
//		Client:         "lighthouse",
//		Network:        "mainnet",
//		DeploymentType: "source",
//		DataDir:        "/data/lighthouse",
//	})
func NewConsensusClientComponent(ctx *pulumi.Context, name string, args *ConsensusClientComponentArgs, opts ...pulumi.ResourceOption) (*ConsensusClientComponent, error) {
	if args == nil {
		args = &ConsensusClientComponentArgs{}
	}

	component := &ConsensusClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:component:ConsensusClient:%s:%s", args.Client, args.Network), name, component, opts...)
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
