package v3

import (
	"context"
	"testing"

	"github.com/imunhatep/awslib/provider/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roles is a two-account pool: enough to tell "scoped to one" from "all".
func testRoles() map[types.AwsAccountID]types.RoleArn {
	return map[types.AwsAccountID]types.RoleArn{
		"111111111111": "arn:aws:iam::111111111111:role/reader",
		"222222222222": "arn:aws:iam::222222222222:role/reader",
	}
}

// A nil ClientBuilder is deliberate throughout: every assertion below must hold
// without AWS being touched, so if the code ever reached the builder these tests
// would panic instead of quietly passing.

func TestPoolAccountIDsComesFromRolesWithoutAws(t *testing.T) {
	pool := NewClientPool(context.Background(), nil, testRoles())

	ids, err := pool.PoolAccountIDs()
	require.NoError(t, err)
	assert.ElementsMatch(t, []types.AwsAccountID{"111111111111", "222222222222"}, ids)
}

// TestGetAccountClientsRejectsUnreachableAccount is the load-bearing one: the
// account check must happen before any client is built, because building a
// client assumes that account's role. A caller scoping to one account must not
// have reached into the others by the time it finds out the account is wrong.
func TestGetAccountClientsRejectsUnreachableAccount(t *testing.T) {
	pool := NewClientPool(context.Background(), nil, testRoles())

	clients, err := pool.GetAccountClients("999999999999", "eu-central-1")

	require.Error(t, err, "an account outside the pool must be an error, not an empty result")
	assert.Empty(t, clients)
	assert.Contains(t, err.Error(), "999999999999")
	assert.Contains(t, err.Error(), "not reachable")
}

// TestGetAccountClientsRejectsEmptyAccount guards the obvious misuse: an empty
// account ID must not silently mean "all accounts".
func TestGetAccountClientsRejectsEmptyAccount(t *testing.T) {
	pool := NewClientPool(context.Background(), nil, testRoles())

	_, err := pool.GetAccountClients("", "eu-central-1")
	require.Error(t, err)
}

// Note: there is deliberately no test that an in-pool account proceeds to build
// a client. collectClients builds them in goroutines, so a nil-builder panic
// there cannot be recovered by the caller and would abort the test binary rather
// than fail an assertion. The three tests above pin the property that matters —
// rejection happens before the AWS boundary — and the aws-mcp-go side covers the
// success path against a real pool.
