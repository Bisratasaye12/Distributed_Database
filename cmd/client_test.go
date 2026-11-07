package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeNode answers like a distsql node and records the last request body.
func fakeNode(t *testing.T, routes map[string]string) (*httptest.Server, *[]string) {
	t.Helper()
	var lastBody []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &lastBody)
		}
		reply, ok := routes[r.Method+" "+r.URL.RequestURI()]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)

	return srv, &lastBody
}

func TestExecSQLSendsStatements(t *testing.T) {
	srv, body := fakeNode(t, map[string]string{
		"POST /execute": `{"results":[{},{"last_insert_id":1,"rows_affected":1}]}`,
	})

	stmts := []string{"CREATE TABLE t (id INTEGER)", "INSERT INTO t VALUES (1)"}
	results, err := execSQL(srv.URL, stmts)

	require.NoError(t, err)
	require.Equal(t, stmts, *body)
	require.Len(t, results, 2)
	require.Equal(t, int64(1), results[1].RowsAffected)
}

func TestPrintExecResultsFailsWhenAStatementFails(t *testing.T) {
	var out bytes.Buffer
	err := printExecResults(&out, []string{"good", "bad"}, []execResult{
		{RowsAffected: 1},
		{Error: "no such table: nope"},
	})

	require.EqualError(t, err, "1 of 2 statements failed")
	require.Contains(t, out.String(), "OK     good")
	require.Contains(t, out.String(), "no such table: nope")
}

func TestQuerySQLStrongAddsConsistencyParam(t *testing.T) {
	srv, _ := fakeNode(t, map[string]string{
		"POST /query?consistency=strong": `{"results":[{"columns":["n"],"types":["integer"],"values":[[2]]}]}`,
	})

	results, err := querySQL(srv.URL, []string{"SELECT count(*) AS n FROM t"}, true)

	require.NoError(t, err)
	require.Equal(t, []interface{}{float64(2)}, results[0].Values[0])
}

func TestPrintQueryResultsTable(t *testing.T) {
	var out bytes.Buffer
	err := printQueryResults(&out, []queryResult{{
		Columns: []string{"id", "name"},
		Values:  [][]interface{}{{float64(1), "abebe"}, {float64(2), nil}},
	}})

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Equal(t, []string{"id  name", "1   abebe", "2   NULL", "(2 rows)"}, lines)
}

func TestRowCount(t *testing.T) {
	require.Equal(t, "(1 row)", rowCount(1))
	require.Equal(t, "(0 rows)", rowCount(0))
}

func TestListServers(t *testing.T) {
	srv, _ := fakeNode(t, map[string]string{
		"GET /servers": `{"servers":[{"id":"node1","rpc_addr":"127.0.0.1:8946","is_leader":true},{"id":"node2","rpc_addr":"127.0.0.1:8947"}]}`,
	})

	servers, err := listServers(srv.URL)
	require.NoError(t, err)

	var out bytes.Buffer
	printServers(&out, servers)
	require.Contains(t, out.String(), "node1    127.0.0.1:8946  leader")
	require.Contains(t, out.String(), "node2    127.0.0.1:8947  follower")
}

func TestServerErrorsIncludeTheMessage(t *testing.T) {
	srv, _ := fakeNode(t, map[string]string{})

	_, err := listServers(srv.URL)

	require.ErrorContains(t, err, "404")
	require.ErrorContains(t, err, "not found")
}

func TestTopLevelErrorField(t *testing.T) {
	srv, _ := fakeNode(t, map[string]string{
		"POST /execute": `{"error":"not leader"}`,
	})

	_, err := execSQL(srv.URL, []string{"INSERT INTO t VALUES (1)"})
	require.EqualError(t, err, "not leader")
}
