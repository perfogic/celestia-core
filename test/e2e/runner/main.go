package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cometbft/cometbft/libs/log"
	e2e "github.com/cometbft/cometbft/test/e2e/pkg"
	"github.com/cometbft/cometbft/test/e2e/pkg/infra"
	"github.com/cometbft/cometbft/test/e2e/pkg/infra/docker"
)

const randomSeed = 2308084734268

var logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))

func main() {
	NewCLI().Run()
}

// CLI is the Cobra-based command-line interface.
type CLI struct {
	root     *cobra.Command
	testnet  *e2e.Testnet
	preserve bool
	infp     infra.Provider
}

// NewCLI sets up the CLI.
func NewCLI() *CLI {
	cli := &CLI{}
	cli.root = &cobra.Command{
		Use:           "runner",
		Short:         "End-to-end test runner",
		SilenceUsage:  true,
		SilenceErrors: true, // we'll output them ourselves in Run()
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			file, err := cmd.Flags().GetString("file")
			if err != nil {
				return err
			}
			m, err := e2e.LoadManifest(file)
			if err != nil {
				return err
			}

			inft, err := cmd.Flags().GetString("infrastructure-type")
			if err != nil {
				return err
			}

			var ifd e2e.InfrastructureData
			switch inft {
			case "docker":
				var err error
				ifd, err = e2e.NewDockerInfrastructureData(m)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown infrastructure type '%s'", inft)
			}

			testnet, err := e2e.LoadTestnet(file, ifd)
			if err != nil {
				return fmt.Errorf("loading testnet: %s", err)
			}

			cli.testnet = testnet
			switch inft {
			case "docker":
				cli.infp = &docker.Provider{
					ProviderData: infra.ProviderData{
						Testnet:            testnet,
						InfrastructureData: ifd,
					},
				}
			default:
				return fmt.Errorf("bad infrastructure type: %s", inft)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := Cleanup(cli.testnet); err != nil {
				return err
			}
			if err := Setup(cli.testnet, cli.infp); err != nil {
				return err
			}

			r := rand.New(rand.NewSource(randomSeed)) //nolint: gosec

			chLoadResult := make(chan error)
			ctx, loadCancel := context.WithCancel(context.Background())
			defer loadCancel()
			go func() {
				err := Load(ctx, cli.testnet)
				if err != nil {
					logger.Error(fmt.Sprintf("Transaction load failed: %v", err.Error()))
				}
				chLoadResult <- err
			}()

			if err := Start(cmd.Context(), cli.testnet, cli.infp); err != nil {
				return err
			}

			if err := Wait(cmd.Context(), cli.testnet, 5); err != nil { // allow some txs to go through
				return err
			}

			if cli.testnet.HasPerturbations() {
				if err := Perturb(cmd.Context(), cli.testnet); err != nil {
					return err
				}
				if err := Wait(cmd.Context(), cli.testnet, 5); err != nil { // allow some txs to go through
					return err
				}
			}

			if cli.testnet.Evidence > 0 {
				if err := InjectEvidence(ctx, r, cli.testnet, cli.testnet.Evidence); err != nil {
					return err
				}
				if err := Wait(cmd.Context(), cli.testnet, 5); err != nil { // ensure chain progress
					return err
				}
			}

			loadCancel()
			if err := <-chLoadResult; err != nil {
				return err
			}
			if err := Wait(cmd.Context(), cli.testnet, 5); err != nil { // wait for network to settle before tests
				return err
			}
			if err := Test(cli.testnet, cli.infp.GetInfrastructureData()); err != nil {
				return err
			}
			if !cli.preserve {
				if err := Cleanup(cli.testnet); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cli.root.PersistentFlags().StringP("file", "f", "", "Testnet TOML manifest")
	_ = cli.root.MarkPersistentFlagRequired("file")

	cli.root.PersistentFlags().StringP("infrastructure-type", "", "docker", "Backing infrastructure used to run the testnet. Only 'docker' is supported")

	cli.root.Flags().BoolVarP(&cli.preserve, "preserve", "p", false,
		"Preserves the running of the test net after tests are completed")

	cli.root.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Generates the testnet directory and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Setup(cli.testnet, cli.infp)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Starts the testnet, waiting for nodes to become available",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := os.Stat(cli.testnet.Dir)
			if os.IsNotExist(err) {
				err = Setup(cli.testnet, cli.infp)
			}
			if err != nil {
				return err
			}
			return Start(cmd.Context(), cli.testnet, cli.infp)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "perturb",
		Short: "Perturbs the testnet, e.g. by restarting or disconnecting nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Perturb(cmd.Context(), cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "wait",
		Short: "Waits for a few blocks to be produced and all nodes to catch up",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Wait(cmd.Context(), cli.testnet, 5)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stops the testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Stopping testnet")
			return cli.infp.StopTestnet(context.Background())
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "load",
		Short: "Generates transaction load until the command is canceled",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return Load(context.Background(), cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "evidence [amount]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generates and broadcasts evidence to a random node",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			amount := 1

			if len(args) == 1 {
				amount, err = strconv.Atoi(args[0])
				if err != nil {
					return err
				}
			}

			return InjectEvidence(
				cmd.Context(),
				rand.New(rand.NewSource(randomSeed)), //nolint: gosec
				cli.testnet,
				amount,
			)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "test",
		Short: "Runs test cases against a running testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Test(cli.testnet, cli.infp.GetInfrastructureData())
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "cleanup",
		Short: "Removes the testnet directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Cleanup(cli.testnet)
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "logs",
		Short: "Shows the testnet logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return docker.ExecComposeVerbose(context.Background(), cli.testnet.Dir, "logs")
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "tail",
		Short: "Tails the testnet logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return docker.ExecComposeVerbose(context.Background(), cli.testnet.Dir, "logs", "--follow")
		},
	})

	cli.root.AddCommand(&cobra.Command{
		Use:   "benchmark",
		Short: "Benchmarks testnet",
		Long: `Benchmarks the following metrics:
	Mean Block Interval
	Standard Deviation
	Min Block Interval
	Max Block Interval
over a 100 block sampling period.
		
Does not run any perturbations.
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := Cleanup(cli.testnet); err != nil {
				return err
			}
			if err := Setup(cli.testnet, cli.infp); err != nil {
				return err
			}

			chLoadResult := make(chan error)
			ctx, loadCancel := context.WithCancel(cmd.Context())
			defer loadCancel()
			go func() {
				err := Load(ctx, cli.testnet)
				if err != nil {
					logger.Error(fmt.Sprintf("Transaction load errored: %v", err.Error()))
				}
				chLoadResult <- err
			}()

			if err := Start(cmd.Context(), cli.testnet, cli.infp); err != nil {
				return err
			}

			if err := Wait(cmd.Context(), cli.testnet, 5); err != nil { // allow some txs to go through
				return err
			}

			// we benchmark performance over the next 100 blocks
			if err := Benchmark(cmd.Context(), cli.testnet, 100); err != nil {
				return err
			}

			loadCancel()
			if err := <-chLoadResult; err != nil {
				return err
			}

			return Cleanup(cli.testnet)
		},
	})

	return cli
}

// Run runs the CLI.
func (cli *CLI) Run() {
	if err := cli.root.Execute(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
