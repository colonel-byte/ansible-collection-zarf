# goreleaser

Builds and publishes a release. Triggered by pushing a tag, which is what [`release-please.yaml`](release-please.yaml.md) does when the release pull request merges.

GoReleaser builds the wrapper binaries and the archives per [`.goreleaser.yaml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.goreleaser.yaml). Its `before` hook shells out to `ansible-galaxy` to build the collection tarball, which is why the job installs `ansible-core` with `pipx` -- without it the release fails at its first step.

The job asks for three writes, each for one reason:

| Permission            | What needs it                                   |
| --------------------- | ----------------------------------------------- |
| `contents: write`     | Creating the release and uploading the archives |
| `id-token: write`     | The OIDC token cosign signs against             |
| `attestations: write` | Proving provenance of the built artifacts       |

Beyond the archives, the workflow publishes provenance. `actions/attest` attests the checksums file, and a second attestation covers an AI-BOM: `.github/ai-bom.json` is rendered with the tag, commit and timestamp filled in by `jq`, then attested and uploaded alongside the release as `dist/ai-bom.json`. Both attestation bundles are uploaded as `.intoto.jsonl` files with `gh release upload --clobber`.

Signing uses cosign with the key and password in the `COSIGN_PRIVATE_KEY` and `COSIGN_PASSWORD` secrets, and Syft is installed for SBOM generation.
