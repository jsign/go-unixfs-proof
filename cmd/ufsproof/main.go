package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	bsrv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipld/go-car/v2/blockstore"
	unixfsproof "github.com/jsign/go-unixfs-proof"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
)

var genProofCmd = &cobra.Command{
	Use:   "prove [cid] [offset]",
	Short: "Generate a proof for a Cid at a specified offset",
	Long: `Generate a proof for a Cid at a specified offset

It can use multiple sources for the underlying Cid data:
- An IPFS API multiaddress storing the Cid DAG (--ipfs-api).
- A CAR file that contains the Cid DAG (--car-path).

Only one of these options should be provided.
`,
	Args: cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		rootCid, err := cid.Decode(args[0])
		if err != nil {
			log.Fatalf("decoding cid: %s", err)
		}
		offset, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			log.Fatalf("parsing offset: %s", err)
		}

		ipfsAPIFlag, err := c.Flags().GetString("ipfs-api")
		if err != nil {
			log.Fatalf("getting --ipfs-api flag: %s", err)
		}
		carPathFlag, err := c.Flags().GetString("car-path")
		if err != nil {
			log.Fatalf("getting --car-path flag: %s", err)
		}

		var dserv ipld.DAGService
		if carPathFlag != "" {
			dserv, err = wireCARDAGService(carPathFlag)
		} else if ipfsAPIFlag != "" {
			dserv, err = wireIPFSAPIDAGService(ipfsAPIFlag)
		} else {
			log.Fatalf("at least one flag --ipfs-api or --car-path should be provided")
		}
		if err != nil {
			log.Fatalf("wiring dagservice: %s", err)
		}

		proof, err := unixfsproof.CreateProof(c.Context(), rootCid, offset, dserv)
		if err != nil {
			log.Fatalf("generating proof: %s", err)
		}
		io.Copy(os.Stdout, bytes.NewReader(proof))
	},
}

var verifyProofCmd = &cobra.Command{
	Use:   "verify [cid] [offset] [proof-path (optional, if not provided is stdin)]",
	Short: "Verifies a generated proof",
	Long:  "Verifies a generated proof",
	Args:  cobra.RangeArgs(2, 3),
	Run: func(c *cobra.Command, args []string) {
		rootCid, err := cid.Decode(args[0])
		if err != nil {
			log.Fatalf("decoding cid: %s", err)
		}
		offset, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			log.Fatalf("parsing offset: %s", err)
		}

		proofFile := os.Stdin
		if len(args) == 3 {
			f, err := os.Open(args[2])
			if err != nil {
				log.Fatalf("opening proof file: %s", err)
			}
			defer f.Close()
			proofFile = f

		}
		proof, err := io.ReadAll(proofFile)
		if err != nil {
			log.Fatalf("reading proof file: %s", err)
		}

		ok, err := unixfsproof.ValidateProof(c.Context(), rootCid, offset, proof)
		if err != nil {
			log.Fatalf("verifying proof: %s", err)
		}
		if ok {
			fmt.Println("The proof is valid")
			return
		}
		fmt.Println("The proof is NOT invalid")
	},
}

var rootCmd = &cobra.Command{
	Use:   "ufsproof",
	Short: "ufsproof allows to generate offset-based UnixFS DAG file proofs",
	Long:  "ufsproof allows to generate offset-based UnixFS DAG file proofs",
	Args:  cobra.ExactArgs(0),
}

func init() {
	genProofCmd.Flags().String("ipfs-api", "/ip4/127.0.0.1/tcp/5001", "IPFS API URL")
	genProofCmd.Flags().String("car-path", "", "CAR path")
	rootCmd.AddCommand(genProofCmd)

	rootCmd.AddCommand(verifyProofCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("executing cmd: %s", err)
	}
}

func wireIPFSAPIDAGService(ipfsAPI string) (ipld.DAGService, error) {
	ma, err := multiaddr.NewMultiaddr(ipfsAPI)
	if err != nil {
		return nil, fmt.Errorf("parsing ipfs api multiaddr: %s", err)
	}
	ipfs, err := httpapi.NewApi(ma)
	if err != nil {
		return nil, fmt.Errorf("creating ipfs client: %s", err)
	}

	return ipfs.Dag(), nil
}

func wireCARDAGService(carPath string) (ipld.DAGService, error) {
	bstore, err := blockstore.OpenReadOnly(carPath)
	if err != nil {
		return nil, fmt.Errorf("opening CAR as blockstore: %s", err)
	}

	return dag.NewDAGService(bsrv.New(bstore, offline.Exchange(bstore))), nil
}
