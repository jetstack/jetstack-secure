package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type cluster struct {
	CertificateAuthorityData string `json:"certificate-authority-data"`
	Server                   string `json:"server"`
}
type clusterRecord struct {
	Cluster cluster `json:"cluster"`
	Name    string  `json:"name"`
}
type config struct {
	Clusters []clusterRecord `json:"clusters"`
}

const (
	green  = "🟢"
	orange = "🟠"
	red    = "🔴"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Kubernetes cluster configuration checker 🚀",
	Long: `Preflight is a tool to automatically perform Kubernetes cluster
configuration checks using Open Policy Agent (OPA).

Preflight checks are bundled into Packages`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		fzf := exec.CommandContext(
			ctx, "fzf",
			"--read0",
			"--preview", "echo {}",
			"--preview-window", "right",
			"--with-nth", "1,2",
			"--delimiter", `\n`,
		)
		fzf.Stdout = cmd.OutOrStdout()
		fzf.Stderr = cmd.ErrOrStderr()
		fzfIn, err := fzf.StdinPipe()
		if err != nil {
			return err
		}
		if err := fzf.Start(); err != nil {
			return err
		}
		c := exec.CommandContext(ctx, "kubectl", "config", "view", "--output=json")
		c.Stderr = cmd.ErrOrStderr()
		out, err := c.Output()
		if err != nil {
			return err
		}
		var cfg config
		if err := json.Unmarshal(out, &cfg); err != nil {
			return err
		}
		for _, c := range cfg.Clusters {
			v, err := getKubernetesVersion(ctx, c.Cluster.Server)
			if err != nil {
				return err
			}

			record := []string{
				fmt.Sprintf("%s %s", green, c.Name),
				fmt.Sprintf("Server: %s", c.Cluster.Server),
				fmt.Sprintf("Version: %s", v["gitVersion"]),
			}
			fmt.Fprint(fzfIn, strings.Join(record, "\n")+"\x00")
		}
		if err := fzfIn.Close(); err != nil {
			return err
		}
		if err := fzf.Wait(); err != nil {
			return err
		}
		return nil
	},
}

func getKubernetesVersion(ctx context.Context, server string) (map[string]string, error) {
	var v map[string]string
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server+"/version", http.NoBody)
	if err != nil {
		return nil, err
	}
	cl := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	vBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(vBytes, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func init() {
	for _, command := range rootCmd.Commands() {
		setFlagsFromEnv("PREFLIGHT_", command.PersistentFlags())
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setFlagsFromEnv(prefix string, fs *pflag.FlagSet) {
	set := map[string]bool{}
	fs.Visit(func(f *pflag.Flag) {
		set[f.Name] = true
	})
	fs.VisitAll(func(f *pflag.Flag) {
		// ignore flags set from the commandline
		if set[f.Name] {
			return
		}
		// remove trailing _ to reduce common errors with the prefix, i.e. people setting it to MY_PROG_
		cleanPrefix := strings.TrimSuffix(prefix, "_")
		name := fmt.Sprintf("%s_%s", cleanPrefix, strings.Replace(strings.ToUpper(f.Name), "-", "_", -1))
		if e, ok := os.LookupEnv(name); ok {
			_ = f.Value.Set(e)
		}
	})
}
