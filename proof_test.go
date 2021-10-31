package unixfsproof

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	chunker "github.com/ipfs/go-ipfs-chunker"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-unixfs/importer/balanced"
	h "github.com/ipfs/go-unixfs/importer/helpers"
	testu "github.com/ipfs/go-unixfs/test"
	"github.com/stretchr/testify/require"
)

func TestProofVerify(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dataSize := int64(100000)
	chunkSize := int64(256)
	rootCid, dserv := setupData(t, dataSize, chunkSize)

	tests := []struct {
		proofOffset uint64
		verifOffset uint64
		notOkVerif  bool
		notOkProof  bool
	}{
		// Correct proofs.
		{proofOffset: 40, verifOffset: 40},
		{proofOffset: 500, verifOffset: 500},
		{proofOffset: 6000, verifOffset: 6000},
		{proofOffset: 70000, verifOffset: 70000},
		{proofOffset: uint64(dataSize), verifOffset: uint64(dataSize)},

		// Correct proof due to being in same block
		{proofOffset: 40, verifOffset: 41},
		{proofOffset: 41, verifOffset: 40},

		// Indirectly correct proofs; this should work unless we change
		// the verification to not allow unvisited blocks; not clear if that's
		// entirely useful.
		{proofOffset: 868, verifOffset: 1124},

		// Definitely wrong proofs.
		{proofOffset: 40, verifOffset: 50000, notOkVerif: true},
		{proofOffset: 70000, verifOffset: 10, notOkVerif: true},

		// Offset bigger than file size.
		{proofOffset: uint64(dataSize) + 1, verifOffset: 0, notOkProof: true},
	}

	for _, test := range tests {
		test := test
		tname := fmt.Sprintf("%d %d", test.proofOffset, test.verifOffset)
		t.Run(tname, func(t *testing.T) {
			t.Parallel()

			proof, err := Prove(ctx, rootCid, test.proofOffset, dserv)
			if test.notOkProof {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			ok, err := Verify(ctx, rootCid, test.verifOffset, proof)
			require.NoError(t, err)
			require.Equal(t, !test.notOkVerif, ok)
		})
	}
}

func setupData(t *testing.T, dataSize, chunkSize int64) (cid.Cid, ipld.DAGService) {
	r := rand.New(rand.NewSource(22))
	data := make([]byte, dataSize)
	_, err := io.ReadFull(r, data)
	require.NoError(t, err)
	in := bytes.NewReader(data)
	opts := testu.UseCidV1
	dserv := testu.GetDAGServ()
	dbp := h.DagBuilderParams{
		Dagserv:    dserv,
		Maxlinks:   3,
		CidBuilder: opts.Prefix,
		RawLeaves:  opts.RawLeavesUsed,
	}
	db, err := dbp.New(chunker.NewSizeSplitter(in, chunkSize))
	require.NoError(t, err)
	n, err := balanced.Layout(db)
	require.NoError(t, err)

	return n.Cid(), dserv
}
