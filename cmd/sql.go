package cmd

import (
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <sql> [sql...]",
	Short: "Run write statements (CREATE, INSERT, UPDATE, DELETE)",
	Example: `  distsql exec "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"
  distsql exec "INSERT INTO users(name) VALUES('abebe')" "INSERT INTO users(name) VALUES('sara')"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := execSQL(addrFlag(cmd), args)
		if err != nil {
			return err
		}
		return printExecResults(cmd.OutOrStdout(), args, results)
	},
}

var queryCmd = &cobra.Command{
	Use:   "query <sql> [sql...]",
	Short: "Run read statements and print the rows",
	Example: `  distsql query "SELECT * FROM users"
  distsql query --strong "SELECT count(*) FROM users"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		strong, _ := cmd.Flags().GetBool("strong")
		results, err := querySQL(addrFlag(cmd), args, strong)
		if err != nil {
			return err
		}
		return printQueryResults(cmd.OutOrStdout(), results)
	},
}

func init() {
	queryCmd.Flags().Bool("strong", false, "read through the leader so the result includes every committed write")
	rootCmd.AddCommand(execCmd, queryCmd)
}
