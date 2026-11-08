package expr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_ConfigDirIsHiddenFromExpressions(t *testing.T) {
	ctx := NewContext().WithConfigDir("/etc/booster")

	_, err := Str("${ ConfigDir }").Resolve(ctx)

	require.Error(t, err)
	assert.Equal(t, "/etc/booster", ctx.ConfigDir)
}
