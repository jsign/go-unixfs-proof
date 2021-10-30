package unixfsproof

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"

	chunker "github.com/ipfs/go-ipfs-chunker"
	"github.com/ipfs/go-unixfs/importer/balanced"
	h "github.com/ipfs/go-unixfs/importer/helpers"
	testu "github.com/ipfs/go-unixfs/test"
	"github.com/stretchr/testify/require"
)

func TestProofVerify(t *testing.T) {
	t.Parallel()

	dserv := testu.GetDAGServ()
	r := rand.New(rand.NewSource(22))
	data := make([]byte, 100000)
	_, err := io.ReadFull(r, data)
	require.NoError(t, err)
	in := bytes.NewReader(data)
	opts := testu.UseCidV1
	dbp := h.DagBuilderParams{
		Dagserv:    dserv,
		Maxlinks:   3,
		CidBuilder: opts.Prefix,
		RawLeaves:  opts.RawLeavesUsed,
	}
	chunkSize := int64(256)
	db, err := dbp.New(chunker.NewSizeSplitter(in, chunkSize))
	require.NoError(t, err)
	node, err := balanced.Layout(db)
	require.NoError(t, err)

	tests := []struct {
		proofOffset uint64
		verifOffset uint64
		ok          bool
	}{
		// Correct proofs.
		{proofOffset: 40, verifOffset: 40, ok: true},
		{proofOffset: 500, verifOffset: 500, ok: true},
		{proofOffset: 6000, verifOffset: 6000, ok: true},
		{proofOffset: 70000, verifOffset: 70000, ok: true},

		// Correct proof due to being in same block
		{proofOffset: 40, verifOffset: 41, ok: true},
		{proofOffset: 41, verifOffset: 40, ok: true},

		// Indirectly correct proofs; this should work unless we change
		// the verification to not allow unvisited blocks; not clear if that's
		// entirely useful.
		{proofOffset: 868, verifOffset: 1124, ok: true},

		// Definitely wrong proofs.
		{proofOffset: 40, verifOffset: 50000, ok: false},
		{proofOffset: 70000, verifOffset: 10, ok: false},
	}

	for _, test := range tests {
		test := test
		tname := fmt.Sprintf("%d %d", test.proofOffset, test.verifOffset)

		t.Run(tname, func(t *testing.T) {
			ctx, cls := context.WithCancel(context.Background())
			defer cls()
			proof, err := CreateProof(ctx, node.Cid(), test.proofOffset, dserv)
			require.NoError(t, err)

			ok, err := ValidateProof(ctx, node.Cid(), test.verifOffset, proof)
			require.NoError(t, err)

			require.Equal(t, test.ok, ok)
		})
	}
}
