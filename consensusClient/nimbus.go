package consensusClient

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

// NewNimbusComponent creates a new consensus client component for Nimbus
// and returns a pointer to the component
//
// Example usage:
//
//	client, err := consensusClient.NewNimbusComponent(ctx, "testNimbusConsensusClient", &consensusClient.ConsensusClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"),             // username for the ssh connection
//			Host:       cfg.Require("sshHost"),             // ip address of the host
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"), // must be a secret, RequireSecret is critical for security
//		},
//		Client:         "nimbus",             // must be "nimbus"
//		Network:        "mainnet",            // mainnet, sepolia, or holesky
//		DeploymentType: "source",             // source, binary, docker
//		DataDir:        "/data/nimbus",	  // path to the data directory
//	})
func NewNimbusComponent(ctx *pulumi.Context, name string, args *ConsensusClientComponentArgs, opts ...pulumi.ResourceOption) (*ConsensusClientComponent, error) {
	if args == nil {
		args = &ConsensusClientComponentArgs{}
	}

	component := &ConsensusClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:componenet:ConsensusClient:%s", args.Client), name, component, opts...)
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
			Create:     pulumi.Sprintf("git clone -b %s %s /data/repos/%s", cfg.Require("nimbusBranch"), cfg.Require("nimbusRepoUrl"), args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error cloning repo", nil)
			return nil, err
		}

		// install pre-reqs
		preReqs, err := remote.NewCommand(ctx, fmt.Sprintf("installPrereqs-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.String("sudo apt install -y git cmake build-essential"),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error installing pre-reqs", nil)
			return nil, err
		}

		// build consensus client
		buildClient, err := remote.NewCommand(ctx, fmt.Sprintf("buildConsensusClient-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("cd /data/repos/%s && sudo -u %s make -j4 nimbus_beacon_node", args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, preReqs}))
		if err != nil {
			ctx.Log.Error("Error building consensus client", nil)
			return nil, err
		}

		// copy binary to /usr/local/bin
		_, err = remote.NewCommand(ctx, fmt.Sprintf("moveClientBinary-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("mv /data/repos/%s/build/nimbus_beacon_node /usr/local/bin/nimbus_beacon_node", args.Client),
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
		serviceDefinition, err := utils.NewServiceDefinitionComponent(ctx, fmt.Sprintf("consensusService-%s", args.Client), &utils.ServiceComponentArgs{
			Connection:  args.Connection,
			ServiceType: args.Client,
			Network:     args.Network,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{buildClient, scriptPerms}))
		if err != nil {
			ctx.Log.Error("Error creating consensus service", nil)
			return nil, err
		}

		// group permissions
		_, err = remote.NewCommand(ctx, fmt.Sprintf("setDataDirGroupPermissions-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R %s:%s %s && chown %s:%s /data/scripts/start_%s.sh", args.Client, args.Client, args.DataDir, args.Client, args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{serviceDefinition, scriptPerms, startScript}))
		if err != nil {
			ctx.Log.Error("Error setting group permissions", nil)
			return nil, err
		}

	} else if args.DeploymentType == Kubernetes {
		storageSize := pulumi.String(args.PodStorageSize) // 30Gi size for holesky
		_, err = corev1.NewPersistentVolumeClaim(ctx, "nimbus-data", &corev1.PersistentVolumeClaimArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("nimbus-data"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("nimbus-data"),
					"app.kubernetes.io/part-of": pulumi.String("nimbus"),
				},
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
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("execution-jwt"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name": pulumi.String("execution-jwt"),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a ConfigMap with the content of nimbus.toml
		nimbusTomlData, err := os.ReadFile(args.ConsensusClientConfigPath)
		if err != nil {
			return nil, err
		}
		nimbusConfigData, err := corev1.NewConfigMap(ctx, "nimbus-config", &corev1.ConfigMapArgs{
			Data: pulumi.StringMap{
				"nimbus.toml": pulumi.String(string(nimbusTomlData)),
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("nimbus-config"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("nimbus-config"),
					"app.kubernetes.io/part-of": pulumi.String("nimbus"),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a stateful set to run a nimbus node with a configmap volume and a data persistent volume
		_, err = appsv1.NewStatefulSet(ctx, "nimbus-set", &appsv1.StatefulSetArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("nimbus"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("nimbus-set"),
					"app.kubernetes.io/part-of": pulumi.String("nimbus"),
				},
			},
			Spec: &appsv1.StatefulSetSpecArgs{
				Replicas: pulumi.Int(1),
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: pulumi.StringMap{
						"app": pulumi.String("nimbus"),
					},
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: pulumi.StringMap{
							"app":                       pulumi.String("nimbus"),
							"app.kubernetes.io/name":    pulumi.String("nimbus"),
							"app.kubernetes.io/part-of": pulumi.String("nimbus"),
						},
					},
					Spec: &corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							corev1.ContainerArgs{
								Name:    pulumi.String("nimbus"),
								Image:   pulumi.String(args.ConsensusClientImage),
								Command: pulumi.ToStringArray(args.ConsensusClientContainerCommands),
								Ports: corev1.ContainerPortArray{
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(9000),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(9000),
										Protocol:      pulumi.String("UDP"),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(9001),
										Protocol:      pulumi.String("UDP"),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(5054),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(5052),
									},
								},
								VolumeMounts: corev1.VolumeMountArray{
									corev1.VolumeMountArgs{
										Name:      pulumi.String("nimbus-config"),
										MountPath: pulumi.String("/etc/nimbus"),
									},
									corev1.VolumeMountArgs{
										Name:      pulumi.String("nimbus-data"),
										MountPath: pulumi.String("/root/.local/share/nimbus/holesky"),
									},
									corev1.VolumeMountArgs{
										Name:      pulumi.String("execution-jwt"),
										MountPath: pulumi.String("/secrets"),
									},
								},
								Resources: &corev1.ResourceRequirementsArgs{
									Limits: pulumi.StringMap{
										"cpu":    pulumi.String(args.CpuLimit),
										"memory": pulumi.String(args.MemoryLimit),
									},
									Requests: pulumi.StringMap{
										"cpu":    pulumi.String(args.CpuRequest),
										"memory": pulumi.String(args.MemoryRequest),
									},
								},
							},
						},
						DnsPolicy: pulumi.String("ClusterFirst"),
						Volumes: corev1.VolumeArray{
							corev1.VolumeArgs{
								Name: pulumi.String("nimbus-config"),
								ConfigMap: &corev1.ConfigMapVolumeSourceArgs{
									Name: nimbusConfigData.Metadata.Name(),
								},
							},
							corev1.VolumeArgs{
								Name: pulumi.String("nimbus-data"),
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSourceArgs{
									ClaimName: pulumi.String("nimbus-data"),
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

		// Create ingress for nimbus p2p traffic on port 9000
		_, err = corev1.NewService(ctx, "nimbus-p2p-service", &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.String("nimbus")},
				Type:     pulumi.String("NodePort"),
				Ports: corev1.ServicePortArray{
					corev1.ServicePortArgs{
						Port: pulumi.Int(9000),
						Name: pulumi.String("p2p-tcp"),
					},
					corev1.ServicePortArgs{
						Port:     pulumi.Int(9000),
						Protocol: pulumi.String("UDP"),
						Name:     pulumi.String("p2p-udp"),
					},
				},
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("nimbus-p2p-service"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("nimbus-p2p-service"),
					"app.kubernetes.io/part-of": pulumi.String("nimbus"),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

	}

	return component, nil

}
