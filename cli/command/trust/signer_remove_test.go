package trust

import (
	"context"
	"io"
	"testing"

	"github.com/docker/cli/internal/test"
	notaryfake "github.com/docker/cli/internal/test/notary"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/tuf/data"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestTrustSignerRemoveErrors(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "not-enough-args-0",
			expectedError: "requires at least 2 arguments",
		},
		{
			name:          "not-enough-args-1",
			args:          []string{"user"},
			expectedError: "requires at least 2 arguments",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newSignerRemoveCommand(
				test.NewFakeCli(&fakeClient{}))
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			assert.ErrorContains(t, cmd.Execute(), tc.expectedError)
		})
	}
	testCasesWithOutput := []struct {
		name           string
		args           []string
		expectedError  string
		expectedErrOut string
	}{
		{
			name:           "not-an-image",
			args:           []string{"user", "notanimage"},
			expectedError:  "error removing signer from: notanimage",
			expectedErrOut: "error retrieving signers for notanimage",
		},
		{
			name:           "sha-reference",
			args:           []string{"user", "870d292919d01a0af7e7f056271dc78792c05f55f49b9b9012b6d89725bd9abd"},
			expectedError:  "error removing signer from: 870d292919d01a0af7e7f056271dc78792c05f55f49b9b9012b6d89725bd9abd",
			expectedErrOut: "invalid repository name",
		},
		{
			name:           "invalid-img-reference",
			args:           []string{"user", "ALPINE"},
			expectedError:  "error removing signer from: ALPINE",
			expectedErrOut: "invalid reference format",
		},
	}
	for _, tc := range testCasesWithOutput {
		t.Run(tc.name, func(t *testing.T) {
			cli := test.NewFakeCli(&fakeClient{})
			cli.SetNotaryClient(notaryfake.GetOfflineNotaryRepository)
			cmd := newSignerRemoveCommand(cli)
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			assert.Check(t, is.Error(err, tc.expectedError))
			assert.Check(t, is.Contains(cli.ErrBuffer().String(), tc.expectedErrOut))
		})
	}
}

func TestRemoveSingleSigner(t *testing.T) {
	cli := test.NewFakeCli(&fakeClient{})
	cli.SetNotaryClient(notaryfake.GetLoadedNotaryRepository)
	ctx := context.Background()
	removed, err := removeSingleSigner(ctx, cli, "signed-repo", "test", true)
	assert.Error(t, err, "no signer test for repository signed-repo")
	assert.Equal(t, removed, false, "No signer should be removed")

	removed, err = removeSingleSigner(ctx, cli, "signed-repo", "releases", true)
	assert.Error(t, err, "releases is a reserved keyword and cannot be removed")
	assert.Equal(t, removed, false, "No signer should be removed")
}

func TestRemoveMultipleSigners(t *testing.T) {
	cli := test.NewFakeCli(&fakeClient{})
	cli.SetNotaryClient(notaryfake.GetLoadedNotaryRepository)
	ctx := context.Background()
	err := removeSigner(ctx, cli, signerRemoveOptions{signer: "test", repos: []string{"signed-repo", "signed-repo"}, forceYes: true})
	assert.Error(t, err, "error removing signer from: signed-repo, signed-repo")
	assert.Check(t, is.Contains(cli.ErrBuffer().String(),
		"no signer test for repository signed-repo"))
	assert.Check(t, is.Contains(cli.OutBuffer().String(), "Removing signer \"test\" from signed-repo...\n"))
}

func TestRemoveLastSignerWarning(t *testing.T) {
	cli := test.NewFakeCli(&fakeClient{})
	ctx := context.Background()
	cli.SetNotaryClient(notaryfake.GetLoadedNotaryRepository)

	err := removeSigner(ctx, cli, signerRemoveOptions{signer: "alice", repos: []string{"signed-repo"}, forceYes: false})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(cli.OutBuffer().String(),
		"The signer \"alice\" signed the last released version of signed-repo. "+
			"Removing this signer will make signed-repo unpullable. "+
			"Are you sure you want to continue? [y/N]"))
}

func TestIsLastSignerForReleases(t *testing.T) {
	role := data.Role{}
	releaserole := client.RoleWithSignatures{}
	releaserole.Name = releasesRoleTUFName
	releaserole.Threshold = 1
	allrole := []client.RoleWithSignatures{releaserole}
	lastsigner, err := isLastSignerForReleases(role, allrole)
	assert.Error(t, err, "all signed tags are currently revoked, use docker trust sign to fix")
	assert.Check(t, is.Equal(false, lastsigner))

	role.KeyIDs = []string{"deadbeef"}
	sig := data.Signature{}
	sig.KeyID = "deadbeef"
	releaserole.Signatures = []data.Signature{sig}
	releaserole.Threshold = 1
	allrole = []client.RoleWithSignatures{releaserole}
	lastsigner, err = isLastSignerForReleases(role, allrole)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, lastsigner))

	sig.KeyID = "8badf00d"
	releaserole.Signatures = []data.Signature{sig}
	releaserole.Threshold = 1
	allrole = []client.RoleWithSignatures{releaserole}
	lastsigner, err = isLastSignerForReleases(role, allrole)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, lastsigner))
}
