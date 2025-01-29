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

// NewRethComponent creates a new reth execution client component
// and the necessary infrastructure to run it.
//
// Example usage:
//
//	client, err := executionClient.NewRethComponent(ctx, "testRethExecutionClient", &executionClient.ExecutionClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"), // username for the ssh connection
//			Host:       cfg.Require("sshHost"), // ip address of the host
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"), // must be a secret, RequireSecret is critical for security
//		},
//		Client:         "reth", // must be "reth"
//		Network:        "mainnet", // mainnet, sepolia, or holesky
//		DeploymentType: "source", // source, kubernetes
//		DataDir:        "/data/mainnet/reth", // path to the data directory
//	})
func NewRethExExComponent(ctx *pulumi.Context, name string, args *ExecutionClientComponentArgs, opts ...pulumi.ResourceOption) (*ExecutionClientComponent, error) {
	cfg := config.New(ctx, "")

	if args == nil {
		args = &ExecutionClientComponentArgs{}
	}

	component := &ExecutionClientComponent{}
	err := ctx.RegisterComponentResource(fmt.Sprintf("custom:component:ExecutionClient:%s", args.Name), name, component, opts...)
	if err != nil {
		return nil, err
	}

	if args.DeploymentType == Source {
		// Execute a sequence of commands on the remote server
		_, err = remote.NewCommand(ctx, fmt.Sprintf("createDataDir-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("mkdir -p %s", args.DataDir),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error creating data directory", nil)
			return nil, err
		}
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

		// script permissions
		_, err = remote.NewCommand(ctx, fmt.Sprintf("scriptPermissions-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chmod +x /data/scripts/start_%s_%s.sh", args.Client, args.Network),
			Delete:     pulumi.String("echo 0"),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{startScript}))
		if err != nil {
			ctx.Log.Error("Error setting script permissions", nil)
			return nil, err
		}

		// Execute a sequence of commands on the remote serve`r
		repo, err := remote.NewCommand(ctx, fmt.Sprintf("cloneRepo-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("git clone -b %s %s /data/repos/%s/reth", cfg.Require("rethGitBranch"), cfg.Require("rethRepoURL"), args.Network),
			Update:     pulumi.String("cd /data/repos/reth && git pull"),
			Delete:     pulumi.Sprintf("rm -rf /data/repos/%s/reth", args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error cloning repo", nil)
			return nil, err
		}

		// set group permissions
		ownership, err := remote.NewCommand(ctx, fmt.Sprintf("setGroupPermissions-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R reth:reth /data/repos/%s/reth", args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, startScript}))
		if err != nil {
			ctx.Log.Error("Error setting group permissions", nil)
			return nil, err
		}

		// install rust toolchain
		rustToolchain, err := remote.NewCommand(ctx, fmt.Sprintf("installRust-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.String("sudo -u reth curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sudo -u reth sh -s -- -y"),
			Connection: args.Connection,
		}, pulumi.Parent(component))
		if err != nil {
			ctx.Log.Error("Error installing rust toolchain", nil)
			return nil, err
		}
		rethInstallation := &remote.Command{}
		if args.Network == "base" {
			rethInstallation, err = remote.NewCommand(ctx, fmt.Sprintf("installReth-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/reth/bin/reth --bin op-reth --features \"optimism\" --root /data", args.Connection.User, args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain, ownership}))
			if err != nil {
				ctx.Log.Error("Error installing reth", nil)
				return nil, err
			}
		} else if args.Network == "sepolia" {
			rethInstallation, err = remote.NewCommand(ctx, fmt.Sprintf("installReth-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/reth/bin/reth --bin reth", args.Connection.User, args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain, ownership}))
			if err != nil {
				ctx.Log.Error("Error installing reth", nil)
				return nil, err
			}
			_, err := remote.NewCommand(ctx, fmt.Sprintf("moveAndRename-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("mv /root/.cargo/bin/reth /data/bin/reth-%s", args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{rethInstallation}))
			if err != nil {
				ctx.Log.Error("Error moving and renaming reth", nil)
				return nil, err
			}
		} else if args.Network == "holesky" {
			rethInstallation, err = remote.NewCommand(ctx, fmt.Sprintf("installReth-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/reth/bin/reth --bin reth", args.Connection.User, args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain, ownership}))
			if err != nil {
				ctx.Log.Error("Error installing reth", nil)
				return nil, err
			}
			_, err := remote.NewCommand(ctx, fmt.Sprintf("moveAndRename-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("mv /root/.cargo/bin/reth /data/bin/reth-%s", args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{rethInstallation}))
			if err != nil {
				ctx.Log.Error("Error moving and renaming reth", nil)
				return nil, err
			}
		} else {

			rethInstallation, err = remote.NewCommand(ctx, fmt.Sprintf("installReth-%s", args.Network), &remote.CommandArgs{
				Create:     pulumi.Sprintf("/%s/.cargo/bin/cargo install --locked --path /data/repos/%s/reth/bin/reth --bin reth --root /data", args.Connection.User, args.Network),
				Connection: args.Connection,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, rustToolchain, ownership}))
			if err != nil {
				ctx.Log.Error("Error installing reth", nil)
				return nil, err
			}
		}

		// group permissions
		groupPerms, err := remote.NewCommand(ctx, fmt.Sprintf("setDataDirGroupPermissions-%s", args.Network), &remote.CommandArgs{
			Create:     pulumi.Sprintf("chown -R %s:%s %s && chown %s:%s /data/bin/%s && chown %s:%s /data/scripts/start_%s_%s.sh", args.Client, args.Client, args.DataDir, args.Client, args.Client, args.Client, args.Client, args.Client, args.Client, args.Network),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{repo, startScript, rethInstallation}))
		if err != nil {
			ctx.Log.Error("Error setting group permissions", nil)
			return nil, err
		}

		if args.Network == "base" {
			_, err = utils.NewServiceDefinitionComponent(ctx, fmt.Sprintf("rethBaseService-%s", args.Network), &utils.ServiceComponentArgs{
				Connection:  args.Connection,
				ServiceType: args.Network,
				Network:     args.Network,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{groupPerms, rethInstallation}))
			if err != nil {
				ctx.Log.Error("Error creating reth service", nil)
				return nil, err
			}
		} else {
			_, err = utils.NewServiceDefinitionComponent(ctx, fmt.Sprintf("rethService-%s", args.Network), &utils.ServiceComponentArgs{
				Connection:  args.Connection,
				ServiceType: args.Client,
				Network:     args.Network,
			}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{groupPerms, rethInstallation}))
			if err != nil {
				ctx.Log.Error("Error creating reth service", nil)
				return nil, err
			}
		}
	} else if args.DeploymentType == Kubernetes {
		// Define static string variables
		rethDataVolumeName := pulumi.Sprintf("%s-config-data", args.Name)
		rethTomlData, err := os.ReadFile(args.ExecutionClientConfigPath)
		if err != nil {
			return nil, err
		}

		// Create a ConfigMap with the content of reth.toml
		configMap, err := corev1.NewConfigMap(ctx, fmt.Sprintf("%s-config", args.Name), &corev1.ConfigMapArgs{
			Data: pulumi.StringMap{
				"reth.toml": pulumi.String(string(rethTomlData)),
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s-config", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-config", args.Name),
					"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Define the PersistentVolumeClaim for reth datadir
		if args.RethSnapshotName != "" {
			storageSize := pulumi.String(args.PodStorageSize)
			_, err = corev1.NewPersistentVolumeClaim(ctx, fmt.Sprintf("%s-data", args.Name), &corev1.PersistentVolumeClaimArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name: rethDataVolumeName,
					Labels: pulumi.StringMap{
						"app.kubernetes.io/name":    rethDataVolumeName,
						"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
					},
				},
				Spec: &corev1.PersistentVolumeClaimSpecArgs{
					AccessModes: pulumi.StringArray{pulumi.String("ReadWriteOnce")},
					DataSource: &corev1.TypedLocalObjectReferenceArgs{
						Kind:     pulumi.String("VolumeSnapshot"),
						Name:     pulumi.String(args.RethSnapshotName),
						ApiGroup: pulumi.String("snapshot.storage.k8s.io"),
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
			storageSize := pulumi.String(args.PodStorageSize)
			_, err = corev1.NewPersistentVolumeClaim(ctx, fmt.Sprintf("%s-data", args.Name), &corev1.PersistentVolumeClaimArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name: rethDataVolumeName,
					Labels: pulumi.StringMap{
						"app.kubernetes.io/name":    rethDataVolumeName,
						"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
					},
				},
				Spec: &corev1.PersistentVolumeClaimSpecArgs{
					AccessModes: pulumi.StringArray{pulumi.String("ReadWriteOnce")},
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

		if args.ExExSnapshotName != "" {
			// Define PersistentVolumeClaim for the execution extension local storage/db
			_, err = corev1.NewPersistentVolumeClaim(ctx, fmt.Sprintf("%s-persistent-storage", args.Name), &corev1.PersistentVolumeClaimArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name: pulumi.Sprintf("%s-persistent-storage", args.Name),
					Labels: pulumi.StringMap{
						"app.kubernetes.io/name":    pulumi.Sprintf("%s-persistent-storage", args.Name),
						"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
					},
				},
				Spec: &corev1.PersistentVolumeClaimSpecArgs{
					AccessModes: pulumi.StringArray{pulumi.String("ReadWriteOnce")},
					DataSource: &corev1.TypedLocalObjectReferenceArgs{
						Kind:     pulumi.String("VolumeSnapshot"),
						Name:     pulumi.String(args.ExExSnapshotName),
						ApiGroup: pulumi.String("snapshot.storage.k8s.io"),
					},
					Resources: &corev1.VolumeResourceRequirementsArgs{
						Requests: pulumi.StringMap{
							"storage": pulumi.String(args.ExExStorageSize),
						},
					},
					StorageClassName: pulumi.String(args.PodStorageClass),
				},
			}, pulumi.Parent(component))
			if err != nil {
				return nil, err
			}
		} else {
			_, err = corev1.NewPersistentVolumeClaim(ctx, fmt.Sprintf("%s-persistent-storage", args.Name), &corev1.PersistentVolumeClaimArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name: pulumi.Sprintf("%s-persistent-storage", args.Name),
					Labels: pulumi.StringMap{
						"app.kubernetes.io/name":    pulumi.Sprintf("%s-persistent-storage", args.Name),
						"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
					},
				},
				Spec: &corev1.PersistentVolumeClaimSpecArgs{
					AccessModes: pulumi.StringArray{pulumi.String("ReadWriteOnce")},
					Resources: &corev1.VolumeResourceRequirementsArgs{
						Requests: pulumi.StringMap{
							"storage": pulumi.String(args.ExExStorageSize),
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

		rethEnvConfigMap, err := corev1.NewConfigMap(ctx, fmt.Sprintf("%s-env-config", args.Name), &corev1.ConfigMapArgs{
			Data: pulumi.ToStringMap(args.Environment),
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s-env-config", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-env-config", args.Name),
					"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
				},
			},
		})
		if err != nil {
			return nil, err
		}

		// Define the StatefulSet for the 'reth' container with a configmap volume and a data persistent volume
		_, err = appsv1.NewStatefulSet(ctx, fmt.Sprintf("%s-set", args.Name), &appsv1.StatefulSetArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s", args.Name),
				Labels: pulumi.StringMap{
					"app":                       pulumi.Sprintf("%s-set", args.Name),
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-set", args.Name),
					"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
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
							"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
						},
					},
					Spec: &corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							corev1.ContainerArgs{
								Name:    pulumi.Sprintf("%s", args.Name),
								Image:   pulumi.String(args.ExecutionClientImage),
								Command: pulumi.ToStringArray(args.ExecutionClientContainerCommands),
								EnvFrom: corev1.EnvFromSourceArray{
									corev1.EnvFromSourceArgs{
										ConfigMapRef: &corev1.ConfigMapEnvSourceArgs{
											Name:     pulumi.Sprintf("%s-env-config", args.Name),
											Optional: pulumi.Bool(true),
										},
									},
								},
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
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(8546),
									},
									corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(8547),
									},
								},
								VolumeMounts: corev1.VolumeMountArray{
									corev1.VolumeMountArgs{
										Name:      pulumi.Sprintf("%s-config", args.Name),
										MountPath: pulumi.String("/etc/reth"),
									},
									corev1.VolumeMountArgs{
										Name:      rethDataVolumeName,
										MountPath: pulumi.String("/root/.local/share/reth"),
									},
									corev1.VolumeMountArgs{
										Name:      pulumi.Sprintf("%s-persistent-storage", args.Name),
										MountPath: pulumi.String("/root/.local/share/exex"),
									},
									corev1.VolumeMountArgs{
										Name:      pulumi.Sprintf("%s-execution-jwt", args.Name),
										MountPath: pulumi.String("/etc/reth/execution-jwt"),
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
						Volumes: corev1.VolumeArray{
							corev1.VolumeArgs{
								Name: pulumi.Sprintf("%s-config", args.Name),
								ConfigMap: &corev1.ConfigMapVolumeSourceArgs{
									Name: configMap.Metadata.Name(),
								},
							},
							corev1.VolumeArgs{
								Name: rethDataVolumeName,
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSourceArgs{
									ClaimName: rethDataVolumeName,
								},
							},
							corev1.VolumeArgs{
								Name: pulumi.Sprintf("%s-execution-jwt", args.Name),
								Secret: &corev1.SecretVolumeSourceArgs{
									SecretName: secret.Metadata.Name(),
								},
							},
							corev1.VolumeArgs{
								Name: pulumi.Sprintf("%s-persistent-storage", args.Name),
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSourceArgs{
									ClaimName: pulumi.Sprintf("%s-persistent-storage", args.Name),
								},
							},
						},
					},
				},
			},
		}, pulumi.DependsOn([]pulumi.Resource{rethEnvConfigMap}), pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a Service for external ports
		_, err = corev1.NewService(ctx, fmt.Sprintf("%s-p2pnet-service", args.Name), &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.Sprintf("%s", args.Name)},
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
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s-p2pnet-service", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-p2pnet-service", args.Name),
					"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a service for internal ports
		_, err = corev1.NewService(ctx, fmt.Sprintf("%s-internal-service", args.Name), &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.Sprintf("%s", args.Name)},
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
				Name: pulumi.Sprintf("%s-internal-service", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-internal-service", args.Name),
					"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create ingress for the reth rpc traffic on port 8545
		_, err = corev1.NewService(ctx, fmt.Sprintf("%s-rpc-service", args.Name), &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.Sprintf("%s", args.Name)},
				Type:     pulumi.String("NodePort"),
				Ports: corev1.ServicePortArray{
					corev1.ServicePortArgs{
						Port:       pulumi.Int(8545),
						TargetPort: pulumi.Int(8545),
					},
				},
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.Sprintf("%s-rpc-service", args.Name),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.Sprintf("%s-rpc-service", args.Name),
					"app.kubernetes.io/part-of": pulumi.Sprintf("%s", args.Name),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}
		return component, nil

	} else if args.DeploymentType == Docker {
		ctx.Log.Error("Docker deployment type not yet supported", nil)
	}

	return component, nil
}
