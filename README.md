# node_deployer

This is a `pulumi` package to deploy ethereum nodes on various platforms. The current implementation allows you to define individual `ConsensusClientComponent` and `ExecutionClientComponent` components and deploy them on a single machine or multiple machines via `pulumi`. The package is designed to be extensible and can be used to any distinct pair of consensus and execution clients. There are also helper functions located in the `utils` directory to help with the deployment of the nodes.

## Requirements

- Requires the following users and groups to exist on the machine:
  - `erigon`
  - `geth`
  - `lighthouse`
  - `lodestar`
  - `nimbus`
  - `prysm`
  - `reth`
  - `teku`

## Example Node Deployment

The following is an example of how to deploy a node with reth for the EL and lighthouse for the CL.

```go
package main

import (
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
    "github.com/pulumi/pulumi-command/sdk/go/command/remote"
    "github.com/rswanson/node_deployer/executionClient" 
    "github.com/rswanson/node_deployer/consensusClient"
)

func main() {
    pulumi.Run(func(ctx *pulumi.Context) error {
        cfg := config.New(ctx, "")

        connection := &remote.ConnectionArgs{
            Host: cfg.Require("host"),
            Port: cfg.Require("port"),
            Username: cfg.Require("username"),
            PrivateKey: cfg.RequireSecret("privateKey"),
        }
        
        // Create a new instance of the reth execution client component
        _, err := executionClient.NewExecutionClientComponent(ctx, "rethExecutionClient", &executionClient.ExecutionClientComponentArgs{
            // Define the execution client component arguments
            Client: "reth",
            Network: "mainnet",
            DataDir: "/data/mainnet/reth",
            DeploymentType: "source",
            Connection: connection,
        })
        if err != nil {
            return err
        }

        // Create a new instance of the lighthouse consensus client component
        _, err = consensusClient.NewConsensusClientComponent(ctx, "lighthouseExecutionClient", &consensusClient.ConsensusClientComponentArgs{
            // Define the consensus client component arguments
            Client: "lighthouse",
            Network: "mainnet",
            DataDir: "/data/mainnet/lighthouse",
            DeploymentType: "source",
            Connection: connection,
        })
        if err != nil {
            return err
        }

        return nil
    })
}
```
