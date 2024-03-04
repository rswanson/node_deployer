package utils

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ServiceDefinitionComponent struct {
	ServiceType string
	pulumi.ResourceState
}

type ServiceComponentArgs struct {
	Connection  *remote.ConnectionArgs
	ServiceType string
}

func NewServiceDefinitionComponent(ctx *pulumi.Context, name string, args *ServiceComponentArgs, opts ...pulumi.ResourceOption) (*ServiceDefinitionComponent, error) {
	if args == nil {
		args = &ServiceComponentArgs{}
	}

	component := &ServiceDefinitionComponent{
		ServiceType: args.ServiceType,
	}
	err := ctx.RegisterComponentResource("custom:resource:ServiceDefinitionComponent", name, component, opts...)
	if err != nil {
		return nil, err
	}

	serviceDefinition, err := remote.NewCopyFile(ctx, fmt.Sprintf("createServiceDefinition-%s", args.ServiceType), &remote.CopyFileArgs{
		LocalPath:  pulumi.Sprintf("config/%s.service", component.ServiceType),
		RemotePath: pulumi.Sprintf("/etc/systemd/system/%s.service", component.ServiceType),
		Connection: args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error copying "+args.ServiceType+" service file", nil)
		return nil, err
	}

	enableService, err := remote.NewCommand(ctx, fmt.Sprintf("enableService-%s", args.ServiceType), &remote.CommandArgs{
		Create:     pulumi.Sprintf("systemctl enable %s", args.ServiceType),
		Connection: args.Connection,
	}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{serviceDefinition}))
	if err != nil {
		ctx.Log.Error("Error enabling "+args.ServiceType+" service", nil)
		return nil, err
	}

	_, err = remote.NewCommand(ctx, fmt.Sprintf("startService-%s", args.ServiceType), &remote.CommandArgs{
		Create:     pulumi.Sprintf("systemctl start %s", args.ServiceType),
		Connection: args.Connection,
	}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{enableService}))
	if err != nil {
		ctx.Log.Error("Error starting "+args.ServiceType+" service", nil)
		return nil, err
	}

	return component, nil
}
