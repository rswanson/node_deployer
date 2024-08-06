package node_deployer

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/rswanson/node_deployer/consensusClient"
	"github.com/rswanson/node_deployer/executionClient"
)

type EthereumNode struct {
	ExecutionClient *executionClient.ExecutionClientComponent
	ConsensusClient *consensusClient.ConsensusClientComponent
	pulumi.ResourceState
}

type EthereumNodeArgs struct {
	ExecutionClientArgs *executionClient.ExecutionClientComponentArgs
	ConsensusClientArgs *consensusClient.ConsensusClientComponentArgs
	Replicas            int
}

func NewEthereumNode(ctx *pulumi.Context, name string, args *EthereumNodeArgs, opts ...pulumi.ResourceOption) (*EthereumNode, error) {
	if args == nil {
		args = &EthereumNodeArgs{}
	}

	executionClient, err := executionClient.NewExecutionClientComponent(ctx, name+"-executionClient", args.ExecutionClientArgs, opts...)
	if err != nil {
		ctx.Log.Error("Error creating execution client", nil)
		return nil, err
	}

	consensusClient, err := consensusClient.NewConsensusClientComponent(ctx, name+"-consensusClient", args.ConsensusClientArgs, opts...)
	if err != nil {
		ctx.Log.Error("Error creating consensus client", nil)
		return nil, err
	}

	return &EthereumNode{
		ExecutionClient: executionClient,
		ConsensusClient: consensusClient,
	}, nil
}
