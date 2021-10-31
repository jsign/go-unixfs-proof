# go-unixfs-proof

Go implementation of offset-based native UnixFS proofs.

**Note:** this is a side-project not used in production. It's mostly in alpha version. It isn't optimized at any level nor audited in any way. 

## Table of contents
- [About the project](#about)
- [Assumptions of the UnixFS DAG file](#Assumptions-of-the-UnixFS-DAG-file)
- [Proof format](#proof-format)
- [Use-case analysis and security](#use-case-analysis-and-security)
- [Proof sizes and benchmark](#proof-sizes-and-benchmark)
- [CLI](#cli)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)
- [Contact](#contact)


## About
This library allows generating and verification proofs for UnixFS file DAGs.

The challenger knows the _Cid_ of a UnixFS DAG and the maximum size of the underlying represented file. This information asks the prover to generate proof that it stores the block at a specified offset between _[0, max-file-size]_.

The proof is a sub-DAG of the original, which contains the path to the targeted block, plus each level of intermediate nodes.

Consider the following UnixFS DAG file with a fanout factor of 3:
![image](https://user-images.githubusercontent.com/6136245/139512869-5135649f-dc34-4ef1-9862-5c47860ec581.png)
<!---
(https://excalidraw.com/#json=5662906028916736,qzS2x9JgfY30Vy2tbzWwiA)
-->


Considering a verifer is asking a prover to provide a proof that it contains the corresponding block at the _file level offset_ X, the prover generates the subdag inside the green zone:
- Round nodes are internal DAG nodes that are somewhat small-ish and don't contain file data.
- Square nodes contain chunks of the original file data.
- The indigo colored nodes are necessary nodes to make the proof verify that the target block (red) is at the specified offset.


## Assumptions of the UnixFS DAG file
This library works with any file UnixFS DAG. It doesn't assume any particular layout (e.g., balanced, trickle, etc.), chunking (e.g., fixed size, etc.), or other particular DAG builder configuration.

## Proof format
To avoid inventing any new proof standard or format, the proof is a byte array. This byte array is a CAR file format of all the blocks that are part of the proof.

Today this is the decided format mostly to avoid friction about defining other formats. The order of blocks in the CAR file should be considered undefined despite the current implementation having a BFS order.

## Use-case analysis and security
The primary motivation is to support a random-sampling-based challenge system between a prover and a verifier.

Given a file with size _MaxSize_, the verifying can ask the prover to generate proof with the underlying block for a specified _Cid_.

The security of this schema is similar to other random-sampling schemas:
- If the underlying prover doesn't have the block, it won't generate the proof.
- If the offset is random-sampled in the _[0, MaxSize]_ range, it can't be guessed by the prover without storing all the files.

If the bad-prover is storing  only part of the leaves _p_ (e.g., 50%):
- A single challenge makes the prover have a probability `p` (e.g., 50%) of success.
- If the challenger asks for N (e.g., 5) proofs, the probability of generating all correct proofs is `p^N` (e.g., 3%) at the cost of a proof size of `SingleProofSize*N`.

If the underlying file has some erasure coding applied with leverage `X` (e.g., 2x):
- A single challenge makes the prover have a probability of `p^X` of success (e.g., 25%)
- If the challenger asks for N (e.g., 5) proofs, the probability of generating all correct proofs is `p^(X*N)` (e.g., 0.097%)

In summary, applying an erasure coding schema in the underlying file can make a single proof be _good enough_ to balance the proof size with more underlying storage for the original file.

Notice that if the prover has missing internal nodes of the UnixFS, then the impact of a missed block is much higher than missing leaves (underlying data) since the probability of hitting an internal node is way bigger than leaves for a random offset. (e.g., if the root Cid block is missing, all challenges will fail). This means that the probability of the prover failing to provide the proofs is lower than the analysis made above for leaves.


## Proof sizes and benchmark
The size of the proof should be already close to the minimal level. Notice that these proofs are pretty big for the single reason that no assumptions are made of DAG layout nor chunking. Thus internal nodes at visited levels include many children. If we're able to have some extra assumptions as fixed-size chunking, then we could potentially ignore untargeted raw leaves which are the biggest in size, and only include the targeted (red) leaf node. For this reason, a proof for an offset could serve for all the left-sided blocks sharing the same parent (**).

Generating and verifying proofs are mostly symmetrical operations. The current implementation is very naive and not optimized in any way. Being stricter with the spec CAR serialization block order can make the implementation faster. Probably, not a big deal unless you're generating proofs for thousands of _Cids_.

## CLI
A simple CLI `ufsproof` is provided which allows to easily generate and verify proofs, which can be installed running `make install`.

To generate proofs, run `ufsproof prove [cid] [offset]` which prints in stdout the proof for block of Cid at the provided offset.
For example:
- `ufsproof prove QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300`: assumes that the Cid is stored in an IPFS API at `/ip4/127.0.0.1/tcp/5001`.
- `ufsproof prove QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300 > proof.car`: stores the proof in a file.
- `ufsproof prove --car-file mydag.car QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300`: uses a CAR file instead of an IPFS API.

To verify proofs, run `ufsproof verify [cid] [offset] [proof-path:(optional, by default stdin)]`.
For example:
- `ufsproof verify QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300 proof.car`


Closing the loop:
```
$ ufsproof prove QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300 | ufsproof verify QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300
The proof is valid
$ ufsproof prove QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 10 | ufsproof verify QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 50000000
The proof is NOT valid
```
Remember that because of (**) mentioned in _Proof sizes and benchmark_ is possible to have a valid proof message on some offsets greater than the proved one.

## Roadmap
The following bullets will probably be implemented soon:
- [ ] Allow direct leaf Cid proof (non-offset based); a bit offtopic for this lib and not sure entirely useful.
- [ ] Benchmarks, may be fun but nothing entirely useful for now.
- [ ] Allow strict mode proof validation; maybe it makes sense to fail faster in some cases, nbd.
- [ ] CLI for validation from DealID in Filecoin network; maybe fun, but `Labels` are unverified.
- [ ] godocs

This is a side-project made for fun, so a priori is a hand-wavy roadmap.

## Contributing

Contributions make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

If you like to donate, please send funds to [this project wallet-address](https://etherscan.io/address/0x2750E75E3771Dfb5041D5014a3dCC6e052fcd575). Any received funds will be directed to other open-source projects or organizations.

## License

Distributed under the MIT License. See `LICENSE` for more information.

## Contact
Ignacio Hagopian - [@jsign](https://github.com/jsign) 
