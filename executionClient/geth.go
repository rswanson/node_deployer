package executionClient

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/rswanson/node_deployer/utils"
)

// NewGethComponent creates a new ExecutionClientComponent resource that represents a geth client
// and the necessary infrastructure to run it.
//
// Example usage:
//
//	client, err := executionClient.NewGethComponent(ctx, "testGethExecutionClient", &executionClient.ExecutionClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"), // username for the ssh connection
//			Host:       cfg.Require("sshHost"), // ip address of the host
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"), // must be a secret, RequireSecret is critical for security
//		},
//		Client:         "geth", // must be "geth"
//		Network:        "mainnet", // mainnet, sepolia, or holesky
//		DeploymentType: "source", // source, binary, docker
//		DataDir:        "/data/mainnet/geth", // path to the data directory
//	})
func NewGethComponent(ctx *pulumi.Context, name string, args *ExecutionClientComponentArgs, opts ...pulumi.ResourceOption) (*ExecutionClientComponent, error) {
	if args == nil {
		args = &ExecutionClientComponentArgs{}
	}

	component := &ExecutionClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:component:ExecutionClient:%s", args.Client), name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Execute a sequence of commands on the remote server
	_, err = remote.NewCommand(ctx, fmt.Sprintf("createDataDir-%s", args.Client), &remote.CommandArgs{
		Create:     pulumi.Sprintf("mkdir -p %s", args.DataDir),
		Connection: args.Connection,
	}, pulumi.Parent(component))
	if err != nil {
		ctx.Log.Error("Error creating data directory", nil)
		return nil, err
	}

	if args.DeploymentType == Source {
		// Load configuration
		cfg := config.New(ctx, "")

		// clone repo
		repo, err := remote.NewCommand(ctx, fmt.Sprintf("cloneRepo-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("git clone -b %s %s /data/repos/%s", cfg.Require("gethBranch"), cfg.Require("gethRepoUrl"), args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error cloning repo", nil)
			return nil, err
		}

		// install go
		goDeps, err := remote.NewCommand(ctx, fmt.Sprintf("installGo-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.String("sudo apt update && sudo apt install -y golang-go"),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error installing go", nil)
			return nil, err
		}

		// set repo permissions
		repoPerms, err := remote.NewCommand(ctx, fmt.Sprintf("setRepoPermissions-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R %s:%s /data/repos/%s", args.Client, args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo}))
		if err != nil {
			ctx.Log.Error("Error setting repo permissions", nil)
			return nil, err
		}

		// build execution client
		buildClient, err := remote.NewCommand(ctx, fmt.Sprintf("buildExecutionClient-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("cd /data/repos/%s && sudo -u %s make geth", args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{goDeps, repoPerms}))
		if err != nil {
			ctx.Log.Error("Error building execution client", nil)
			return nil, err
		}

		// move client binary to /usr/local/bin
		_, err = remote.NewCommand(ctx, fmt.Sprintf("moveClientBinary-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("mv /data/repos/%s/build/bin/geth /usr/local/bin/geth", args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{buildClient}))
		if err != nil {
			ctx.Log.Error("Error moving client binary", nil)
			return nil, err
		}

		// copy start script
		startScript, err := remote.NewCopyFile(ctx, fmt.Sprintf("copyStartScript-%s", args.Client), &remote.CopyFileArgs{
			LocalPath:  pulumi.Sprintf("scripts/start_%s.sh", args.Client),
			RemotePath: pulumi.Sprintf("/data/scripts/start_%s.sh", args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error copying start script", nil)
			return nil, err
		}

		// script permissions
		scriptPerms, err := remote.NewCommand(ctx, fmt.Sprintf("scriptPermissions-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chmod +x /data/scripts/start_%s.sh", args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{startScript}))
		if err != nil {
			ctx.Log.Error("Error setting script permissions", nil)
			return nil, err
		}

		// create service
		serviceDefinition, err := utils.NewServiceDefinitionComponent(ctx, fmt.Sprintf("executionService-%s", args.Client), &utils.ServiceComponentArgs{
			Connection:  args.Connection,
			ServiceType: args.Client,
			Network:     args.Network,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{buildClient, scriptPerms}))
		if err != nil {
			ctx.Log.Error("Error creating execution service", nil)
			return nil, err
		}

		// group permissions
		_, err = remote.NewCommand(ctx, fmt.Sprintf("setDataDirGroupPermissions-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R %s:%s %s && chown %s:%s /data/scripts/start_%s.sh && chown /usr/local/bin/%s", args.Client, args.Client, args.DataDir, args.Client, args.Client, args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{serviceDefinition, scriptPerms, startScript}))
		if err != nil {
			ctx.Log.Error("Error setting group permissions", nil)
			return nil, err
		}

	} else if args.DeploymentType == Kubernetes {
		// Define static string variables
		gethDataVolumeName := pulumi.String("geth-config-data")
		gethTomlData, err := os.ReadFile(args.ExecutionClientConfigPath)
		if err != nil {
			return nil, err
		}

		// Create a ConfigMap with the content of geth.toml
		configMap, err := corev1.NewConfigMap(ctx, "geth-config", &corev1.ConfigMapArgs{
			Data: pulumi.StringMap{
				"geth.toml": pulumi.String(string(gethTomlData)),
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Define the PersistentVolumeClaim for 1.5TB storage
		storageSize := pulumi.String(args.PodStorageSize) // 30Gi size for holesky
		_, err = corev1.NewPersistentVolumeClaim(ctx, "geth-data", &corev1.PersistentVolumeClaimArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: gethDataVolumeName,
			},
			Spec: &corev1.PersistentVolumeClaimSpecArgs{
				AccessModes: pulumi.StringArray{pulumi.String("ReadWriteOnce")}, // This should match your requirements
				Resources: &corev1.VolumeResourceRequirementsArgs{
					Requests: pulumi.StringMap{
						"storage": storageSize,
					},
				},
				StorageClassName: pulumi.String(args.PodStorageClass),
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a secret for the execution jwt
		secret, err := corev1.NewSecret(ctx, "execution-jwt", &corev1.SecretArgs{
			StringData: pulumi.StringMap{
				"jwt.hex": pulumi.String(args.ExecutionJwt),
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Define the StatefulSet for the 'geth' container with a configmap volume and a data persistent volume
		_, err = appsv1.NewStatefulSet(ctx, "geth-set", &appsv1.StatefulSetArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("geth"),
			},
			Spec: &appsv1.StatefulSetSpecArgs{
				Replicas: pulumi.Int(1),
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: pulumi.StringMap{
						"app": pulumi.String("geth"),
					},
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: pulumi.StringMap{
							"app": pulumi.String("geth"),
						},
					},
					Spec: &corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							corev1.ContainerArgs{
								Name:    pulumi.String("geth"),
								Image:   pulumi.String(args.ExecutionClientImage),
								Command: pulumi.ToStringArray(args.ExecutionClientContainerCommands),
								Ports: corev1.ContainerPortArray{
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(30303),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(30303),
										Protocol:      pulumi.String("UDP"),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(9001),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(8545),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(8551),
									},
								},
								VolumeMounts: corev1.VolumeMountArray{
									corev1.VolumeMountArgs{
										Name:      pulumi.String("geth-config"),
										MountPath: pulumi.String("/etc/geth"),
									},
									corev1.VolumeMountArgs{
										Name:      gethDataVolumeName,
										MountPath: pulumi.String("/root/.local/share/geth"),
									},
									corev1.VolumeMountArgs{
										Name:      pulumi.String("execution-jwt"),
										MountPath: pulumi.String("/etc/geth/execution-jwt"),
									},
								},
							},
						},
						Volumes: corev1.VolumeArray{
							corev1.VolumeArgs{
								Name: pulumi.String("geth-config"),
								ConfigMap: &corev1.ConfigMapVolumeSourceArgs{
									Name: configMap.Metadata.Name(),
								},
							},
							corev1.VolumeArgs{
								Name: gethDataVolumeName,
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSourceArgs{
									ClaimName: gethDataVolumeName,
								},
							},
							corev1.VolumeArgs{
								Name: pulumi.String("execution-jwt"),
								Secret: &corev1.SecretVolumeSourceArgs{
									SecretName: secret.Metadata.Name(),
								},
							},
						},
					},
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a Service for external ports
		_, err = corev1.NewService(ctx, "geth-p2pnet-service", &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.String("geth")},
				Type:     pulumi.String("NodePort"),
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Port: pulumi.Int(30303),
						Name: pulumi.String("p2p-tcp"),
					},
					&corev1.ServicePortArgs{
						Port:     pulumi.Int(30303),
						Protocol: pulumi.String("UDP"),
						Name:     pulumi.String("p2p-udp"),
					},
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a service for internal ports
		_, err = corev1.NewService(ctx, "geth-internal-service", &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.String("geth")},
				Type:     pulumi.String("ClusterIP"),
				Ports: corev1.ServicePortArray{
					corev1.ServicePortArgs{
						Port: pulumi.Int(9001),
						Name: pulumi.String("metrics"),
					},
					corev1.ServicePortArgs{
						Port: pulumi.Int(8551),
						Name: pulumi.String("p2p"),
					},
				},
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("geth-internal-service"),
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create ingress for the geth rpc traffic on port 8545
		_, err = corev1.NewService(ctx, "geth-rpc-service", &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.String("geth")},
				Type:     pulumi.String("NodePort"),
				Ports: corev1.ServicePortArray{
					corev1.ServicePortArgs{
						Port:       pulumi.Int(8545),
						TargetPort: pulumi.Int(8545),
					},
				},
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("geth-rpc-service"),
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}
		return component, nil

	}

	return component, nil

}
