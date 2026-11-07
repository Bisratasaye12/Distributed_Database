package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Bisratasaye12/Distributed_Database/pb"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type execResult struct {
	LastInsertID int64  `json:"last_insert_id"`
	RowsAffected int64  `json:"rows_affected"`
	Error        string `json:"error"`
}

type queryResult struct {
	Columns []string        `json:"columns"`
	Types   []string        `json:"types"`
	Values  [][]interface{} `json:"values"`
	Error   string          `json:"error"`
}

// do sends a request and decodes a JSON reply into out. Non-2xx replies
// become errors that include the server's message.
func do(method, url string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s %s: %s: %s", method, url, resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unexpected response from %s: %w", url, err)
	}
	return nil
}

func execSQL(addr string, statements []string) ([]execResult, error) {
	var resp struct {
		Results []execResult `json:"results"`
		Error   string       `json:"error"`
	}
	if err := do(http.MethodPost, addr+"/execute", statements, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Results, nil
}

func querySQL(addr string, statements []string, strong bool) ([]queryResult, error) {
	url := addr + "/query"
	if strong {
		url += "?consistency=strong"
	}

	var resp struct {
		Results []queryResult `json:"results"`
		Error   string        `json:"error"`
	}
	if err := do(http.MethodPost, url, statements, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Results, nil
}

func listServers(addr string) ([]*pb.Server, error) {
	var resp struct {
		Servers []*pb.Server `json:"servers"`
	}
	if err := do(http.MethodGet, addr+"/servers", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Servers, nil
}

// printExecResults prints one line per statement and returns an error if
// any statement failed, so scripts can rely on the exit code.
func printExecResults(w io.Writer, statements []string, results []execResult) error {
	failed := 0
	for i, r := range results {
		stmt := ""
		if i < len(statements) {
			stmt = statements[i]
		}
		if r.Error != "" {
			failed++
			fmt.Fprintf(w, "ERROR  %s\n       %s\n", stmt, r.Error)
			continue
		}
		fmt.Fprintf(w, "OK     %s (rows affected: %d, last insert id: %d)\n", stmt, r.RowsAffected, r.LastInsertID)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d statements failed", failed, len(results))
	}
	return nil
}

func printQueryResults(w io.Writer, results []queryResult) error {
	failed := 0
	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if r.Error != "" {
			failed++
			fmt.Fprintf(w, "ERROR  %s\n", r.Error)
			continue
		}

		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, strings.Join(r.Columns, "\t"))
		for _, row := range r.Values {
			cells := make([]string, len(row))
			for j, v := range row {
				cells[j] = formatValue(v)
			}
			fmt.Fprintln(tw, strings.Join(cells, "\t"))
		}
		tw.Flush()
		fmt.Fprintln(w, rowCount(len(r.Values)))
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d queries failed", failed, len(results))
	}
	return nil
}

func rowCount(n int) string {
	if n == 1 {
		return "(1 row)"
	}
	return fmt.Sprintf("(%d rows)", n)
}

func printServers(w io.Writer, servers []*pb.Server) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NODE ID\tADDRESS\tROLE")
	for _, s := range servers {
		role := "follower"
		if s.IsLeader {
			role = "leader"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Id, s.RpcAddr, role)
	}
	tw.Flush()
}

func formatValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case float64:
		// JSON numbers decode as float64; show whole numbers without ".0"
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
