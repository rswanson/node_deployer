package utils

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ServiceDefinitionComponent struct {
	pulumi.ResourceState
}

type ServiceComponentArgs struct {
	Connection  *remote.ConnectionArgs
	Network     string
	ServiceType string
}

func NewServiceDefinitionComponent(ctx *pulumi.Context, name string, args *ServiceComponentArgs, opts ...pulumi.ResourceOption) (*ServiceDefinitionComponent, error) {
	if args == nil {
		args = &ServiceComponentArgs{}
	}

	component := &ServiceDefinitionComponent{}
	err := ctx.RegisterComponentResource("custom:resource:ServiceDefinitionComponent", name, component, opts...)
	if err != nil {
		return nil, err
	}

	serviceDefinition, err := remote.NewCopyFile(ctx, fmt.Sprintf("createServiceDefinition-%s-%s", args.ServiceType, args.Network), &remote.CopyFileArgs{
		LocalPath:  pulumi.Sprintf("config/%s.%s.service", args.ServiceType, args.Network),
		RemotePath: pulumi.Sprintf("/etc/systemd/system/%s.%s.service", args.ServiceType, args.Network),
		Connection: args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error copying "+args.ServiceType+" service file", nil)
		return nil, err
	}

	enableService, err := remote.NewCommand(ctx, fmt.Sprintf("enableService-%s-%s", args.ServiceType, args.Network), &remote.CommandArgs{
		Create:     pulumi.Sprintf("systemctl enable %s", fmt.Sprintf("%s.%s", args.ServiceType, args.Network)),
		Delete:     pulumi.Sprintf("systemctl disable %s", fmt.Sprintf("%s.%s", args.ServiceType, args.Network)),
		Connection: args.Connection,
	}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{serviceDefinition}))
	if err != nil {
		ctx.Log.Error("Error enabling "+args.ServiceType+" service", nil)
		return nil, err
	}

	_, err = remote.NewCommand(ctx, fmt.Sprintf("startService-%s-%s", args.ServiceType, args.Network), &remote.CommandArgs{
		Create:     pulumi.Sprintf("systemctl start %s", fmt.Sprintf("%s.%s", args.ServiceType, args.Network)),
		Delete:     pulumi.Sprintf("systemctl stop %s", fmt.Sprintf("%s.%s", args.ServiceType, args.Network)),
		Connection: args.Connection,
	}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{enableService}))
	if err != nil {
		ctx.Log.Error("Error starting "+args.ServiceType+" service", nil)
		return nil, err
	}

	return component, nil
}
