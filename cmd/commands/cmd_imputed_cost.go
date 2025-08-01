package commands

import (
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/urfave/cli"
)

var queryImputedCostManagerCommand = cli.Command{
	Name:     "queryicm",
	Category: "Imputed Costs", // TODO: subcommand for imputed costs?
	Usage:    "Query the internal state of the imputed cost manager.",
	Description: `
        Query the internal state of the imputed cost manager.
	`,
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name: "namespace",
			Usage: "(optional) name of the namespace. This flag " +
				"can be specified multiple times in the same " +
				"command.",
		},
	},
	Action: actionDecorator(queryImputedCostManager),
}

func queryImputedCostManager(ctx *cli.Context) error {
	ctxc := getContext()
	conn := getClientConn(ctx, false)
	defer conn.Close()

	client := routerrpc.NewRouterClient(conn)

	req := &routerrpc.QueryImputedCostsRequest{}
	namespaces := ctx.StringSlice("namespace")
	if len(namespaces) > 0 {
		req.Namespaces = namespaces
	}

	snapshot, err := client.XQueryImputedCosts(ctxc, req)
	if err != nil {
		return err
	}

	printRespJSON(snapshot)

	return nil
}
