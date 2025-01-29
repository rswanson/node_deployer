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

// NewPrysmComponent creates a new instance of the ConsensusClientComponent for Prysm
// It returns a pointer to the ConsensusClientComponent and an error
//
// Example usage:
//
//	client, err := consensusClient.NewPrysmComponent(ctx, "testPrysmConsensusClient", &consensusClient.ConsensusClientComponentArgs{
//		Connection:     &remote.ConnectionArgs{
//			User:       cfg.Require("sshUser"), 		    // username for the ssh connection
//			Host:       cfg.Require("sshHost"),             // ip address of the host
//			PrivateKey: cfg.RequireSecret("sshPrivateKey"), // must be a secret, RequireSecret is critical for security
//		},
//		Client:         "prysm", 			// must be "prysm"
//		Network:        "mainnet", 			// mainnet, sepolia, or holesky
//		DeploymentType: "source", 			// source, binary, docker
//		DataDir:        "/data/prysm", 		// path to the data directory
//	})

func NewPrysmComponent(ctx *pulumi.Context, name string, args *ConsensusClientComponentArgs, opts ...pulumi.ResourceOption) (*ConsensusClientComponent, error) {
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
			Create:     pulumi.Sprintf("git clone -b %s %s /data/repos/%s", cfg.Require("prysmBranch"), cfg.Require("prysmRepoUrl"), args.Client),
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

		// build consensus client
		buildClient, err := remote.NewCommand(ctx, fmt.Sprintf("buildConsensusClient-%s", args.Client), &remote.CommandArgs{
			Create:     pulumi.Sprintf("cd /data/repos/%s && sudo -u %s make", args.Client, args.Client),
			Connection: args.Connection,
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{goDeps, repoPerms}))
		if err != nil {
			ctx.Log.Error("Error building consensus client", nil)
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
		_, err = corev1.NewPersistentVolumeClaim(ctx, "prysm-data", &corev1.PersistentVolumeClaimArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("prysm-data"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("prysm-data"),
					"app.kubernetes.io/part-of": pulumi.String("prysm"),
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

		// Create a ConfigMap with the content of prysm.toml
		prysmTomlData, err := os.ReadFile(args.ConsensusClientConfigPath)
		if err != nil {
			return nil, err
		}
		prysmConfigData, err := corev1.NewConfigMap(ctx, "prysm-config", &corev1.ConfigMapArgs{
			Data: pulumi.StringMap{
				"prysm.toml": pulumi.String(string(prysmTomlData)),
			},
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("prysm-config"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("prysm-config"),
					"app.kubernetes.io/part-of": pulumi.String("prysm"),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

		// Create a stateful set to run a prysm node with a configmap volume and a data persistent volume
		_, err = appsv1.NewStatefulSet(ctx, "prysm-set", &appsv1.StatefulSetArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("prysm"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("prysm-set"),
					"app.kubernetes.io/part-of": pulumi.String("prysm"),
				},
			},
			Spec: &appsv1.StatefulSetSpecArgs{
				Replicas: pulumi.Int(1),
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: pulumi.StringMap{
						"app": pulumi.String("prysm"),
					},
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: pulumi.StringMap{
							"app":                       pulumi.String("prysm"),
							"app.kubernetes.io/name":    pulumi.String("prysm"),
							"app.kubernetes.io/part-of": pulumi.String("prysm"),
						},
					},
					Spec: &corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							corev1.ContainerArgs{
								Name:    pulumi.String("prysm"),
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
										Name:      pulumi.String("prysm-config"),
										MountPath: pulumi.String("/etc/prysm"),
									},
									corev1.VolumeMountArgs{
										Name:      pulumi.String("prysm-data"),
										MountPath: pulumi.String("/root/.local/share/prysm/holesky"),
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
								Name: pulumi.String("prysm-config"),
								ConfigMap: &corev1.ConfigMapVolumeSourceArgs{
									Name: prysmConfigData.Metadata.Name(),
								},
							},
							corev1.VolumeArgs{
								Name: pulumi.String("prysm-data"),
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSourceArgs{
									ClaimName: pulumi.String("prysm-data"),
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

		// Create ingress for prysm p2p traffic on port 9000
		_, err = corev1.NewService(ctx, "prysm-p2p-service", &corev1.ServiceArgs{
			Spec: &corev1.ServiceSpecArgs{
				Selector: pulumi.StringMap{"app": pulumi.String("prysm")},
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
				Name: pulumi.String("prysm-p2p-service"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("prysm-p2p-service"),
					"app.kubernetes.io/part-of": pulumi.String("prysm"),
				},
			},
		}, pulumi.Parent(component))
		if err != nil {
			return nil, err
		}

	}

	return component, nil

}
