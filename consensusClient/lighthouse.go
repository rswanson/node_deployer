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

// NewLighthouseComponent creates a new consensus client component for Lighthouse
// and returns a pointer to the component
//
// Example usage:
//
//	client, err := consensusClient.NewLighthouseComponent(ctx, "testLighthouseConsensusClient", &consensusClient.ConsensusClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"),             // username for the ssh connection
//			Host:       cfg.Require("sshHost"),             // ip address of the host
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"), // must be a secret, RequireSecret is critical for security
//		},
//		Client:         "lighthouse",         // must be "lighthouse"
//		Network:        "mainnet",            // mainnet, sepolia, or holesky
//		DeploymentType: "source",             // source, binary, docker
//		DataDir:        "/data/lighthouse",	  // path to the data directory
//	})
func NewLighthouseComponent(ctx *pulumi.Context, name string, args *ConsensusClientComponentArgs, opts ...pulumi.ResourceOption) (*ConsensusClientComponent, error) {
	if args == nil {
		args = &ConsensusClientComponentArgs{}
	}

	component := &ConsensusClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:componenet:ConsensusClient:%s", args.Name), name, component, opts...)
	if err != nil {
		return nil, err
	}

	if args.DeploymentType == Source {
		// Load configuration
		cfg := config.New(ctx, "")

		// Execute a sequence of commands on the remote server
		_, err = remote.NewCommand(ctx, fmt.Sprintf("createDataDir-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("mkdir -p %s", args.DataDir),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating data directory", nil)
			return nil, err
		}

		// clone repo
		repo, err := remote.NewCommand(ctx, fmt.Sprintf("cloneRepo-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("git clone -b %s %s /data/repos/%s/%s", cfg.Require("lighthouseBranch"), cfg.Require("lighthouseRepoURL"), args.Network, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error cloning repo", nil)
			return nil, err
		}

		// install rust toolchain
		rustToolchain, err := remote.NewCommand(ctx, fmt.Sprintf("installRust-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.String("curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y"),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error installing rust toolchain", nil)
			return nil, err
		}

		// build consensus client
		buildClient, err := remote.NewCommand(ctx, fmt.Sprintf("buildConsensusClient-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/lighthouse/lighthouse --bin lighthouse --root /data", args.Connection.User, args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain}))
		if err != nil {
			ctx.Log.Error("Error building consensus client", nil)
			return nil, err
		}

		// copy start script
		startScript, err := remote.NewCopyFile(ctx, fmt.Sprintf("copyStartScript-%s", args.Client), &remote.CopyFileArgs{
			LocalPath:  pulumi.Sprintf("scripts/start_%s_%s.sh", args.Client, args.Network),
			RemotePath: pulumi.Sprintf("/data/scripts/start_%s_%s.sh", args.Client, args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error copying start script", nil)
			return nil, err
		}

		// script permissions
		scriptPerms, err := remote.NewCommand(ctx, fmt.Sprintf("scriptPermissions-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chmod +x /data/scripts/start_%s_%s.sh", args.Client, args.Network),
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
			Create:     pulumi.Sprintf("chown -R %s:%s %s && chown %s:%s /data/bin/%s && chown %s:%s /data/scripts/start_%s_%s.sh", args.Client, args.Client, args.DataDir, args.Client, args.Client, args.Client, args.Client, args.Client, args.Client, args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{serviceDefinition, scriptPerms, startScript}))
		if err != nil {
			ctx.Log.Error("Error setting group permissions", nil)
			return nil, err
		}
	} else if args.DeploymentType == Docker {
		ctx.Log.Info("Docker deployment not yet implemented", nil)
	} else if args.DeploymentType == Kubernetes {
		storageSize := pulumi.String(args.PodStorageSize) // 30Gi size for holesky

		if args.SnapshotName != "" {

			_, err = corev1.NewPersistentVolumeClaim(ctx, fmt.Sprintf("%s-data", args.Name), &corev1.PersistentVolumeClaimArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name: pulumi.Sprintf("%s-data", args.Name),
					Labels: pulumi.StringMap{
						"app.kubernetes.io/name":    pulumi.Sprintf("%s-data", args.Name),
						"app.kubernetes.io/part-of": pulumi.String("lighthouse"),
					},
				},
				Spec: &corev1.PersistentVolumeClaimSpecArgs{
					AccessModes: pulumi.StringArray{pulumi.String("ReadWriteOnce")}, // This should match your requirements
					DataSource: &corev1.TypedLocalObjectReferenceArgs{
						Kind: pulumi.String("PersistentVolumeSnapshot"),
						Name: pulumi.String(args.SnapshotName),
					},
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
		} else {
			_, err = corev1.NewPersistentVolumeClaim(ctx, fmt.Sprintf("%s-data", args.Name), &corev1.PersistentVolumeClaimArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name: pulumi.Sprintf("%s-data", args.Name),
					Labels: pulumi.StringMap{
						"app.kubernetes.io/name":    pulumi.Sprintf("%s-data", args.Name),
						"app.kubernetes.io/part-of": pulumi.String("lighthouse"),
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
		}

		// Create a secret for the execution jwt
		secret, err := corev1.NewSecret(ctx, fmt.Sprintf("%s-execution-jwt", args.Name), &corev1.SecretArgs{
			StringData: pulumi.StringMap{
				"jwt.hex": pulumi.String(args.ExecutionJwt),
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s-execution-jwt", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name": pulumi.Sprintf("%s-execution-jwt", args.Name),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a ConfigMap with the content of lighthouse.toml
		lighthouseTomlData, err := os.ReadFile(args.ConsensusClientConfigPath)
		if err != nil {
			return nil, err
		}
		lighthouseConfigData, err := corev1.NewConfigMap(ctx, fmt.Sprintf("%s-config", args.Name), &corev1.ConfigMapArgs{
			Data: pulumi.StringMap{
				"lighthouse.toml": pulumi.String(string(lighthouseTomlData)),
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s-config", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-config", args.Name),
					"app.kubernetes.io/part-of": pulumi.String("lighthouse"),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a stateful set to run a lighthouse node with a configmap volume and a data persistent volume
		_, err = appsv1.NewStatefulSet(ctx, fmt.Sprintf("%s-set", args.Name), &appsv1.StatefulSetArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-set", args.Name),
					"app.kubernetes.io/part-of": pulumi.String("lighthouse"),
				},
			},
			Spec: &appsv1.StatefulSetSpecArgs{
				Replicas: pulumi.Int(1),
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: pulumi.StringMap{
						"app": pulumi.Sprintf("%s", args.Name),
					},
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: pulumi.StringMap{
							"app":                       pulumi.Sprintf("%s", args.Name),
							"app.kubernetes.io/name":    pulumi.Sprintf("%s", args.Name),
							"app.kubernetes.io/part-of": pulumi.String("lighthouse"),
						},
					},
					Spec: &corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							corev1.ContainerArgs{
								Name:    pulumi.Sprintf("%s", args.Name),
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
										Name:      pulumi.Sprintf("%s-config", args.Name),
										MountPath: pulumi.String("/etc/lighthouse"),
									},
									corev1.VolumeMountArgs{
										Name:      pulumi.Sprintf("%s-data", args.Name),
										MountPath: pulumi.String("/root/.lighthouse/holesky"),
									},
									corev1.VolumeMountArgs{
										Name:      pulumi.Sprintf("%s-execution-jwt", args.Name),
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
								Name: pulumi.Sprintf("%s-config", args.Name),
								ConfigMap: &corev1.ConfigMapVolumeSourceArgs{
									Name: lighthouseConfigData.Metadata.Name(),
								},
							},
							corev1.VolumeArgs{
								Name: pulumi.Sprintf("%s-data", args.Name),
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSourceArgs{
									ClaimName: pulumi.Sprintf("%s-data", args.Name),
								},
							},
							corev1.VolumeArgs{
								Name: pulumi.Sprintf("%s-execution-jwt", args.Name),
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

		// Create ingress for lighthouse p2p traffic on port 9000
		_, err = corev1.NewService(ctx, fmt.Sprintf("%s-p2p-service", args.Name), &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.Sprintf("%s", args.Name)},
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
				Name: pulumi.Sprintf("%s-p2p-service", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-p2p-service", args.Name),
					"app.kubernetes.io/part-of": pulumi.String("lighthouse"),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// create the metrics service
		_, err = corev1.NewService(ctx, fmt.Sprintf("%s-metrics-service", args.Name), &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.Sprintf("%s", args.Name)},
				Type:     pulumi.String("ClusterIP"),
				Ports: corev1.ServicePortArray{

					corev1.ServicePortArgs{
						Port: pulumi.Int(5054),
						Name: pulumi.String("metrics"),
					},
				},
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s-metrics-service", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-metrics-service", args.Name),
					"app.kubernetes.io/part-of": pulumi.String("lighthouse"),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

	}

	return component, nil
}
