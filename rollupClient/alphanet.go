package rollupClient

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type RollupClientComponent struct {
	pulumi.ResourceState
	Client  string
	Network string
}

type RollupClientComponentArgs struct {
	Connection     *remote.ConnectionArgs
	Client         string
	Network        string
	DeploymentType string
	DataDir        string
}

func NewRollupClientComponent(ctx *pulumi.Context, name string, args *RollupClientComponentArgs, opts ...pulumi.ResourceOption) (*RollupClientComponent, error) {
	if args == nil {
		args = &RollupClientComponentArgs{}
	}

	component := &RollupClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:component:RollupClient:%s", args.Client), name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Load configuration
	cfg := config.New(ctx, "")

	switch args.DeploymentType {
	case "source":

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
	case "kubernetes":
		// Load configuration
		cfg := config.New(ctx, "")

	}

	return component, nil
}
