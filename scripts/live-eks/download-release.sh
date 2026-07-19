#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/live-eks/lib.sh
source "${script_dir}/lib.sh"

need_cmd docker
need_cmd gh
need_cmd python3
require_release_env
mkdirs

archive_suffix="${ARCHIVE_SUFFIX:-linux_amd64}"
archive="kubepreflight_${RELEASE_TAG}_${archive_suffix}.tar.gz"
checksums="kubepreflight_${RELEASE_TAG}_checksums.txt"
sbom="kubepreflight_${RELEASE_TAG}_sbom.spdx.json"
bare_tag="${RELEASE_TAG#v}"

gh release download "${RELEASE_TAG}" \
  --repo "${GH_REPO}" \
  --dir "${LIVE_EKS_RELEASE_DIR}" \
  --pattern "${archive}" \
  --pattern "${checksums}" \
  --pattern "${sbom}"

(
  cd "${LIVE_EKS_RELEASE_DIR}"
  if command -v sha256sum >/dev/null 2>&1; then
    grep -- "${archive}" "${checksums}" | sha256sum -c -
  else
    grep -- "${archive}" "${checksums}" | shasum -a 256 -c -
  fi
  python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d.get('spdxVersion') and d.get('packages'), 'SBOM missing spdxVersion/packages'" "${sbom}"
)

tar -xzf "${LIVE_EKS_RELEASE_DIR}/${archive}" -C "${LIVE_EKS_RELEASE_DIR}"
bin_dir="${LIVE_EKS_RELEASE_DIR}/kubepreflight_${RELEASE_TAG}_${archive_suffix}"
bin="${bin_dir}/kubepreflight"
chmod +x "${bin}"

"${bin}" version >"${LIVE_EKS_RELEASE_DIR}/binary-version.txt"
grep -qx "KubePreflight ${bare_tag}" "${LIVE_EKS_RELEASE_DIR}/binary-version.txt" || die "binary version banner does not match ${bare_tag}"
grep -qx "commit: ${EXPECTED_RELEASE_COMMIT}" "${LIVE_EKS_RELEASE_DIR}/binary-version.txt" || die "binary commit does not match ${EXPECTED_RELEASE_COMMIT}"
grep -q "^built: unknown$" "${LIVE_EKS_RELEASE_DIR}/binary-version.txt" && die "binary build timestamp is unknown"

v_digest="$(resolve_image_digest "${IMAGE_REPOSITORY}:${RELEASE_TAG}")"
bare_digest="$(resolve_image_digest "${IMAGE_REPOSITORY}:${bare_tag}")"
[ "${v_digest}" = "${EXPECTED_IMAGE_DIGEST}" ] || die "${IMAGE_REPOSITORY}:${RELEASE_TAG} digest ${v_digest} != expected ${EXPECTED_IMAGE_DIGEST}"
[ "${bare_digest}" = "${EXPECTED_IMAGE_DIGEST}" ] || die "${IMAGE_REPOSITORY}:${bare_tag} digest ${bare_digest} != expected ${EXPECTED_IMAGE_DIGEST}"

image_ref="${IMAGE_REPOSITORY}@${EXPECTED_IMAGE_DIGEST}"
docker run --rm "${image_ref}" version >"${LIVE_EKS_RELEASE_DIR}/container-version.txt"
diff -u "${LIVE_EKS_RELEASE_DIR}/binary-version.txt" "${LIVE_EKS_RELEASE_DIR}/container-version.txt"

printf '%s\n' "${bin}" >"${LIVE_EKS_RELEASE_DIR}/binary.path"
printf '%s\n' "${image_ref}" >"${LIVE_EKS_RELEASE_DIR}/image.ref"
cat >"${LIVE_EKS_RELEASE_DIR}/provenance.env" <<EOF
RELEASE_TAG=${RELEASE_TAG}
EXPECTED_RELEASE_COMMIT=${EXPECTED_RELEASE_COMMIT}
EXPECTED_IMAGE_DIGEST=${EXPECTED_IMAGE_DIGEST}
ARCHIVE_SUFFIX=${archive_suffix}
RELEASE_BINARY=${bin}
RELEASE_IMAGE=${image_ref}
EOF

echo "OK: release binary and container provenance verified"
echo "binary: ${bin}"
echo "image: ${image_ref}"
