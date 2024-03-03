package executionClient

import (
	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ExecutionClientComponent struct {
	pulumi.ResourceState
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
