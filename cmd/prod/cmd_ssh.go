package prod

import (
	core "dappco.re/go/core"
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/infra/internal/coreexec"
)

var sshCmd = &cli.Command{
	Use:   "ssh <host>",
	Short: "SSH into a production host",
	Long: `Open an SSH session to a production host defined in infra.yaml.

Examples:
  core prod ssh noc
  core prod ssh de
  core prod ssh de2
  core prod ssh build`,
	Args: cli.ExactArgs(1),
	RunE: runSSH,
}

func runSSH(cmd *cli.Command, args []string) error {
	cfg, _, err := loadConfig()
	if err != nil {
		return err
	}

	name := args[0]
	host, ok := cfg.Hosts[name]
	if !ok {
		// List available hosts
		cli.Print("Unknown host '%s'. Available:\n", name)
		for n, h := range cfg.Hosts {
			cli.Print("  %s  %s  (%s)\n", cli.BoldStyle.Render(n), h.IP, h.Role)
		}
		return core.E("prod.ssh", core.Concat("host '", name, "' not found in infra.yaml"), nil)
	}

	sshArgs := []string{
		"-i", host.SSH.Key,
		"-p", core.Sprintf("%d", host.SSH.Port),
		"-o", "StrictHostKeyChecking=accept-new",
		core.Sprintf("%s@%s", host.SSH.User, host.IP),
	}

	cli.Print("%s %s@%s (%s)\n",
		cli.BoldStyle.Render("▶"),
		host.SSH.User, host.FQDN,
		cli.DimStyle.Render(host.IP))

	// Replace current process with SSH
	if err := coreexec.Exec("ssh", sshArgs...); err != nil {
		return core.E("prod.ssh", "exec ssh", err)
	}
	return nil
}
