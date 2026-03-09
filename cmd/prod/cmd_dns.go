package prod

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"forge.lthn.ai/core/cli/pkg/cli"
	"forge.lthn.ai/core/go-infra"
)

var dnsCmd = &cli.Command{
	Use:   "dns",
	Short: "Manage DNS records via CloudNS",
	Long: `View and manage DNS records for host.uk.com via CloudNS API.

Requires:
  CLOUDNS_AUTH_ID        CloudNS auth ID
  CLOUDNS_AUTH_PASSWORD  CloudNS auth password`,
}

var dnsListCmd = &cli.Command{
	Use:   "list [zone]",
	Short: "List DNS records",
	Args:  cli.MaximumNArgs(1),
	RunE:  runDNSList,
}

var dnsSetCmd = &cli.Command{
	Use:   "set <host> <type> <value>",
	Short: "Create or update a DNS record",
	Long: `Create or update a DNS record. Example:
  core prod dns set hermes.lb A 1.2.3.4
  core prod dns set "*.host.uk.com" CNAME hermes.lb.host.uk.com`,
	Args: cli.ExactArgs(3),
	RunE: runDNSSet,
}

var (
	dnsZone string
	dnsTTL  int
)

func init() {
	dnsCmd.PersistentFlags().StringVar(&dnsZone, "zone", "host.uk.com", "DNS zone")

	dnsSetCmd.Flags().IntVar(&dnsTTL, "ttl", 300, "Record TTL in seconds")

	dnsCmd.AddCommand(dnsListCmd)
	dnsCmd.AddCommand(dnsSetCmd)
}

func getDNSClient() (*infra.CloudNSClient, error) {
	authID := os.Getenv("CLOUDNS_AUTH_ID")
	authPass := os.Getenv("CLOUDNS_AUTH_PASSWORD")
	if authID == "" || authPass == "" {
		return nil, errors.New("CLOUDNS_AUTH_ID and CLOUDNS_AUTH_PASSWORD required")
	}
	return infra.NewCloudNSClient(authID, authPass), nil
}

func runDNSList(cmd *cli.Command, args []string) error {
	dns, err := getDNSClient()
	if err != nil {
		return err
	}

	zone := dnsZone
	if len(args) > 0 {
		zone = args[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	records, err := dns.ListRecords(ctx, zone)
	if err != nil {
		return fmt.Errorf("list records: %w", err)
	}

	cli.Print("%s DNS records for %s\n\n", cli.BoldStyle.Render("▶"), cli.TitleStyle.Render(zone))

	if len(records) == 0 {
		cli.Print("  No records found\n")
		return nil
	}

	for id, r := range records {
		cli.Print("  %s  %-6s  %-30s  %s  TTL:%s\n",
			cli.DimStyle.Render(id),
			cli.BoldStyle.Render(r.Type),
			r.Host,
			r.Record,
			r.TTL)
	}

	return nil
}

func runDNSSet(cmd *cli.Command, args []string) error {
	dns, err := getDNSClient()
	if err != nil {
		return err
	}

	host := args[0]
	recordType := args[1]
	value := args[2]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	changed, err := dns.EnsureRecord(ctx, dnsZone, host, recordType, value, dnsTTL)
	if err != nil {
		return fmt.Errorf("set record: %w", err)
	}

	if changed {
		cli.Print("%s %s %s %s -> %s\n",
			cli.SuccessStyle.Render("✓"),
			recordType, host, dnsZone, value)
	} else {
		cli.Print("%s Record already correct\n", cli.DimStyle.Render("·"))
	}

	return nil
}
